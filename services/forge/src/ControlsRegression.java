// Focused real pinned-Forge payment, priority preference, and combat bridge checks.
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
import forge.game.zone.ZoneType;
import forge.game.phase.PhaseType;
import forge.game.mana.*;
import forge.gamemodes.match.input.*;
import forge.player.*;

public final class ControlsRegression {
    static void check(boolean yes,String reason){WorkerRegression.check(yes,reason);}
    public static void main(String[] args)throws Exception {try{run(args);System.exit(0);}catch(Throwable e){e.printStackTrace();System.exit(1);}}
    static JsonObject stop(ForgeWorker b,String request,String seat,PhaseType phase,boolean enabled) {
        JsonObject action=new JsonObject();action.addProperty("request",request);action.addProperty("session",b.incarnation);action.addProperty("action","setPhaseStop");action.addProperty("revision",0);action.addProperty("prompt","");action.addProperty("seat",seat);action.addProperty("phase",phase.name());action.addProperty("enabled",enabled);return action;
    }
    static void run(String[] args)throws Exception {
        Path out=Path.of(args[2]);ForgeWorker b=new ForgeWorker(out);
        GuiBase.setInterface(b.new Desktop());FModel.initialize(null,null);
        var hp=RegisteredPlayer.forCommander(DeckSerializer.fromFile(Path.of(args[0]).toFile())).setPlayer(new LobbyPlayerHuman("Human"));
        var cp=RegisteredPlayer.forCommander(DeckSerializer.fromFile(Path.of(args[1]).toFile())).setPlayer(GamePlayerUtil.createAiPlayer("CPU",0,"Default"));
        GameRules rules=new GameRules(GameType.Commander);rules.setAppliedVariants(EnumSet.of(GameType.Commander));
        b.game=new Match(rules,List.of(hp,cp),"Controls regression").createGame();b.human=b.game.getPlayers().get(0);Player cpu=b.game.getPlayers().get(1);
        b.controller=(PlayerControllerHuman)b.human.getController();b.gui=(IGuiGame)Proxy.newProxyInstance(IGuiGame.class.getClassLoader(),new Class[]{IGuiGame.class},b);b.controller.setGui(b.gui);
        b.game.setAge(GameStage.Play);b.game.getPhaseHandler().devModeSet(PhaseType.MAIN1,b.human);
        Card forest1=ChoiceRegression.make(b,"Forest",ZoneType.Battlefield),forest2=ChoiceRegression.make(b,"Forest",ZoneType.Battlefield);
        Card bear=ChoiceRegression.make(b,"Grizzly Bears",ZoneType.Hand);
        // Actual payment input and controller dispatch: Forge determines whether
        // Auto is affordable and taps the two sources itself.
        var ability=bear.getFirstSpellAbility();ability.setActivatingPlayer(b.human);
        var cost=new ManaCostBeingPaid(bear.getManaCost());
        InputPayManaOfCostPayment payment=new InputPayManaOfCostPayment(b.controller,cost,ability,b.human,null,false);
        var paid=CompletableFuture.runAsync(payment::showAndWait);
        ForgeWorker.Pending pp=WorkerRegression.await(b);
        check(pp.input==payment&&pp.ok&&b.okLabel.equals("Auto"),"Forge Auto was not enabled in payment projection");
        JsonObject auto=WorkerRegression.reply(b,pp,"auto-pay-actual");auto.addProperty("action","ok");b.accept(auto);
        paid.get(10,TimeUnit.SECONDS);SwingUtilities.invokeAndWait(()->{});
        check(payment.isPaid()&&cost.isPaid()&&forest1.isTapped()&&forest2.isTapped(),"Forge automatic payment did not pay and tap sources");
        synchronized(b.gate){b.pending=null;}
        // Both seats' public floating pools remain visible outside a payment prompt.
        b.human.getManaPool().addMana(new Mana(forge.card.MagicColor.GREEN,forest1,null,b.human));
        cpu.getManaPool().addMana(new Mana(forge.card.MagicColor.BLUE,forest2,null,cpu));
        JsonObject poolView=ForgeWorker.JSON.toJsonTree(b.snapshot("input",null)).getAsJsonObject();
        for(int seat=0;seat<2;seat++) {
            JsonArray pool=poolView.getAsJsonArray("players").get(seat).getAsJsonObject().getAsJsonArray("mana");
            check(pool.size()==6,"Mana colors missing");int sum=0;for(JsonElement entry:pool)sum+=entry.getAsJsonObject().get("amount").getAsInt();check(sum==1,"Public floating mana missing");
        }
        // Preference updates are independent of a blocking choice, preserve its
        // identity, validate seats, and reach Forge's own per-turn callback.
        var choice=ChoiceRegression.call(()->b.one("test-choice","Choose explicitly",List.of("A","B"),false));var pending=WorkerRegression.await(b);
        JsonObject disable=stop(b,"stop-cpu-upkeep","CPU",PhaseType.UPKEEP,false);b.accept(disable);
        check(b.pending==pending&&b.pending.revision==pending.revision,"Preference invalidated waiting choice");
        check(b.gui.isUiSetToSkipPhase(cpu.getView(),PhaseType.UPKEEP)&&!b.gui.isUiSetToSkipPhase(b.human.getView(),PhaseType.UPKEEP),"Seat preference crossed seats");
        check(b.accept(disable).contains("true"),"Preference lost idempotent receipt");disable.addProperty("enabled",true);WorkerRegression.reject(b,disable);
        JsonObject stale=stop(b,"stop-old-session","CPU",PhaseType.DRAW,false);stale.addProperty("session","old-session");WorkerRegression.reject(b,stale);
        JsonObject invalid=stop(b,"stop-other-seat","CPU",PhaseType.DRAW,false);invalid.addProperty("seat","Other");WorkerRegression.reject(b,invalid);
        b.accept(WorkerRegression.reply(b,pending,"choice-after-preference",1));check(choice.get(2,TimeUnit.SECONDS).equals("B"),"Preference answered forced choice");SwingUtilities.invokeAndWait(()->{});
        b.accept(stop(b,"stop-main-disabled","Human",PhaseType.MAIN1,false));SwingUtilities.invokeAndWait(()->{});
        var skipped=CompletableFuture.supplyAsync(b.controller::chooseSpellAbilityToPlay);check(skipped.get(2,TimeUnit.SECONDS)==null,"Forge did not skip disabled empty-stack phase");
        b.game.getStack().add(ability);
        var response=CompletableFuture.supplyAsync(b.controller::chooseSpellAbilityToPlay);var responsePrompt=WorkerRegression.await(b);
        check(responsePrompt.input instanceof InputPassPriority,"Disabled phase skipped response to spell on the stack");
        JsonObject pass=WorkerRegression.reply(b,responsePrompt,"explicit-stack-response");pass.addProperty("action","ok");b.accept(pass);response.get(2,TimeUnit.SECONDS);SwingUtilities.invokeAndWait(()->{});
        b.accept(stop(b,"stop-main-enabled","Human",PhaseType.MAIN1,true));SwingUtilities.invokeAndWait(()->{});
        check(!b.gui.isUiSetToSkipPhase(b.human.getView(),PhaseType.MAIN1),"Reenabled phase still skipped");
        // Actual controller combat assignment: legal allocation, invalid trample
        // rejection without consuming the prompt, and a blocker assigning damage.
        Card attacker=ChoiceRegression.make(b,"Colossal Dreadmaw",ZoneType.Battlefield);
        Card blocker1=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Grizzly Bears"),cpu,b.game);cpu.getZone(ZoneType.Battlefield).add(blocker1);
        Card blocker2=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Giant Spider"),cpu,b.game);cpu.getZone(ZoneType.Battlefield).add(blocker2);
        var assignment=ChoiceRegression.call(()->b.controller.assignCombatDamage(attacker,new CardCollection(List.of(blocker1,blocker2)),null,6,cpu,false));
        var damage=WorkerRegression.await(b);check(damage.kind.equals("assignCombatDamage"),"Missing combat allocation");
        JsonObject illegal=WorkerRegression.reply(b,damage,"illegal-trample");illegal.add("amounts",JsonParser.parseString("[0,0,6]"));WorkerRegression.reject(b,illegal);check(b.pending==damage,"Illegal allocation consumed prompt");
        JsonObject legal=WorkerRegression.reply(b,damage,"legal-combat");legal.add("amounts",JsonParser.parseString("[2,4,0]"));b.accept(legal);
        Map<Card,Integer> assigned=assignment.get(2,TimeUnit.SECONDS);check(assigned.get(blocker1)==2&&assigned.get(blocker2)==4&&assigned.get(null)==0,"Forge controller damage mapping changed");
        var blocking=ChoiceRegression.call(()->b.controller.assignCombatDamage(attacker,new CardCollection(List.of(blocker1,blocker2)),null,6,null,false));
        damage=WorkerRegression.await(b);JsonObject blockReply=WorkerRegression.reply(b,damage,"blocker-damage");blockReply.add("amounts",JsonParser.parseString("[2,4]"));b.accept(blockReply);check(blocking.get(2,TimeUnit.SECONDS).size()==2,"Blocker damage failed without defender");
        // The combat overlay references only already-projected public entities.
        var combat=new forge.game.combat.Combat(b.human);combat.addAttacker(attacker,cpu);combat.addBlocker(attacker,blocker1);b.game.getPhaseHandler().setCombat(combat);b.game.updateCombatForView();
        JsonObject combatView=ForgeWorker.JSON.toJsonTree(b.snapshot("input",null)).getAsJsonObject();JsonObject row=combatView.getAsJsonArray("combat").get(0).getAsJsonObject();
        check(row.get("attacker").getAsInt()==b.publicIds.get(attacker.getId())&&row.getAsJsonArray("plannedBlockers").get(0).getAsInt()==b.publicIds.get(blocker1.getId()),"Combat overlay lost public entity mapping");
        check(row.getAsJsonObject("defender").get("type").getAsString().equals("player"),"Missing defending player");
        combat.getBandOfAttacker(attacker).setBlocked(true);b.game.updateCombatForView();
        JsonObject declared=ForgeWorker.JSON.toJsonTree(b.snapshot("input",null)).getAsJsonObject().getAsJsonArray("combat").get(0).getAsJsonObject();
        check(declared.getAsJsonArray("blockers").get(0).getAsInt()==b.publicIds.get(blocker1.getId()),"Declared blocker link absent");
        RuntimeException diagnostic=new IllegalStateException("Private card name must not be retained");diagnostic.setStackTrace(new StackTraceElement[]{new StackTraceElement("ForgeWorker","assignCombat","ForgeWorker.java",1)});
        check(ForgeWorker.failureReference(diagnostic).equals("adapter_failure_assignCombat_IllegalStateException"),"Unsafe or unhelpful failure reference");
        check(!b.blocked,"Adapter blocked during controls regression");
        Files.writeString(out.resolve("checks.json"),ForgeWorker.JSON.toJson(ForgeWorker.map("passed",true,"actualForgeAutoPayment",true,"floatingManaBothSeats",true,"phasePreferencePromptSafety",true,"engineSkipsDisabledPhase",true,"actualControllerCombatDamage",true,"publicCombatLinks",true,"fullNaturalGame",false)));
        System.out.println("PASS Forge automatic payment, public floating mana, phase stop preferences, controller combat damage and public links");
    }
}
