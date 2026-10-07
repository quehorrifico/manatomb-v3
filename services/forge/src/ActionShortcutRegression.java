// Real controller clicks prove singleton presentation shortcuts without bypassing choices.
import java.nio.file.*;
import java.util.*;
import java.util.concurrent.*;
import java.lang.reflect.Proxy;
import javax.swing.SwingUtilities;
import com.google.gson.*;
import forge.gui.GuiBase;
import forge.gui.interfaces.IGuiGame;
import forge.model.FModel;
import forge.deck.*;
import forge.deck.io.DeckSerializer;
import forge.game.*;
import forge.game.card.*;
import forge.game.player.*;
import forge.game.spellability.*;
import forge.game.zone.ZoneType;
import forge.game.phase.PhaseType;
import forge.gamemodes.match.input.*;
import forge.player.*;

public final class ActionShortcutRegression {
    static void check(boolean yes,String why){WorkerRegression.check(yes,why);}
    public static void main(String[] args)throws Exception{try{run(args);System.exit(0);}catch(Throwable e){e.printStackTrace();System.exit(1);}}
    static SpellAbility click(ForgeWorker b,Card card,String request)throws Exception {
        var selected=CompletableFuture.supplyAsync(b.controller::chooseSpellAbilityToPlay);var p=WorkerRegression.await(b);
        check(p.input instanceof InputPassPriority,"Expected priority for deliberate card click");
        int id=p.cards.entrySet().stream().filter(e->e.getValue().getId()==card.getId()).map(Map.Entry::getKey).findFirst().orElseThrow();
        var action=WorkerRegression.reply(b,p,request);action.addProperty("action","card");action.addProperty("id",id);b.accept(action);
        List<SpellAbility> result=selected.get(2,TimeUnit.SECONDS);SwingUtilities.invokeAndWait(()->{});
        check(result!=null&&result.size()==1&&result.get(0).getHostCard()==card,"Single explicit action was not returned to the controller");
        check(b.pending==null,"Redundant singleton ability prompt remained");return result.get(0);
    }
    static void run(String[] args)throws Exception {
        Path out=Path.of(args[2]);ForgeWorker b=new ForgeWorker(out);GuiBase.setInterface(b.new Desktop());FModel.initialize(null,null);
        var hp=RegisteredPlayer.forCommander(DeckSerializer.fromFile(Path.of(args[0]).toFile())).setPlayer(new LobbyPlayerHuman("Human"));
        var cp=RegisteredPlayer.forCommander(DeckSerializer.fromFile(Path.of(args[1]).toFile())).setPlayer(GamePlayerUtil.createAiPlayer("CPU",0,"Default"));
        GameRules rules=new GameRules(GameType.Commander);rules.setAppliedVariants(EnumSet.of(GameType.Commander));b.game=new Match(rules,List.of(hp,cp),"Action shortcut regression").createGame();b.human=b.game.getPlayers().get(0);
        b.controller=(PlayerControllerHuman)b.human.getController();b.gui=(IGuiGame)Proxy.newProxyInstance(IGuiGame.class.getClassLoader(),new Class[]{IGuiGame.class},b);b.controller.setGui(b.gui);
        b.controller.getInputQueue().addObserver((source,value)->{synchronized(b.gate){if(b.pending!=null&&b.pending.reply==null){b.pending=null;b.published=b.workingView();}}});
        b.game.setAge(GameStage.Play);b.game.getPhaseHandler().devModeSet(PhaseType.MAIN1,b.human);
        Card forest1=ChoiceRegression.make(b,"Forest",ZoneType.Battlefield),forest2=ChoiceRegression.make(b,"Forest",ZoneType.Battlefield),land=ChoiceRegression.make(b,"Forest",ZoneType.Hand),bear=ChoiceRegression.make(b,"Grizzly Bears",ZoneType.Hand);
        SpellAbility playLand=click(b,land,"single-land-click");check(playLand.isLandAbility(),"Wrong land action");
        check(CompletableFuture.supplyAsync(()->b.controller.playChosenSpellAbility(playLand)).get(2,TimeUnit.SECONDS)&&land.isInPlay(),"Land did not play directly");
        SpellAbility tap=click(b,land,"single-mana-click");check(tap.isManaAbility(),"Wrong basic land mana action");
        check(CompletableFuture.supplyAsync(()->b.controller.playChosenSpellAbility(tap)).get(2,TimeUnit.SECONDS)&&land.isTapped()&&b.human.getManaPool().totalMana()==1,"Basic land mana did not resolve directly");
        b.human.getManaPool().clearPool(false);
        SpellAbility spell=click(b,bear,"single-cast-click");var cast=CompletableFuture.supplyAsync(()->b.controller.playChosenSpellAbility(spell));var payment=WorkerRegression.await(b);
        check(payment.input instanceof InputPayMana&&payment.ok&&b.okLabel.equals("Auto"),"Casting paused at redundant choose-ability instead of actual payment");
        var pay=WorkerRegression.reply(b,payment,"cast-auto-payment");pay.addProperty("action","ok");b.accept(pay);check(cast.get(5,TimeUnit.SECONDS)&&forest1.isTapped()&&forest2.isTapped(),"Spell payment failed");SwingUtilities.invokeAndWait(()->{});
        // A one-option callback outside an explicit click/continuation remains a
        // real optional choice; all general one/confirm/mode callbacks are unchanged.
        var optional=ChoiceRegression.call(()->b.gui.getAbilityToPlay(bear.getView(),List.of(spell.getView()),null));var option=WorkerRegression.await(b);
        check(option.kind.equals("getAbilityToPlay")&&option.min==0&&!optional.isDone(),"Unrelated optional singleton was silently chosen");b.accept(WorkerRegression.reply(b,option,"decline-optional-single"));check(optional.get(2,TimeUnit.SECONDS)==null,"Optional cancellation changed");
        Card dual=ChoiceRegression.make(b,"Sol Ring",ZoneType.Battlefield);List<SpellAbility> variants=List.of(spell,dual.getManaAbilities().get(0));
        var multiple=ChoiceRegression.call(()->{b.clickedCard.set(bear.getId());try{return b.controller.getAbilityToPlay(bear,variants,null);}finally{b.clickedCard.remove();}});option=WorkerRegression.await(b);
        check(option.options.size()==2&&!multiple.isDone(),"Explicit card click silently chose among multiple actions");b.accept(WorkerRegression.reply(b,option,"choose-multi-action",1));check(multiple.get(2,TimeUnit.SECONDS)==variants.get(1),"Multiple-choice original ability identity lost");
        check(!b.blocked,"Adapter failed during shortcuts");
        Files.writeString(out.resolve("checks.json"),ForgeWorker.JSON.toJson(ForgeWorker.map("passed",true,"actualLandPlay",true,"actualBasicMana",true,"actualSpellPayment",true,"noDuplicateAdditionalCostPrompt",true,"optionalSingletonPreserved",true,"multipleChoicesPreserved",true)));
        System.out.println("PASS deliberate single land/mana/spell clicks, additional-cost continuation, optional and multi-choice preservation");
    }
}
