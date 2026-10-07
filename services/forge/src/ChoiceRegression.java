// Real pinned controller/view regression; browser fixture separately verifies stacking/resolution.
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
import forge.game.trigger.*;
import forge.game.zone.ZoneType;
import forge.game.ability.AbilityKey;
import forge.player.*;

public final class ChoiceRegression {
    static void check(boolean ok,String why){WorkerRegression.check(ok,why);}
    public static void main(String[] args)throws Exception {
        try {run(args);System.exit(0);}catch(Throwable e){e.printStackTrace();System.exit(1);}
    }
    static void run(String[] args)throws Exception {
        Path out=Path.of(args[2]);Files.createDirectories(out);ForgeWorker b=new ForgeWorker(out);
        GuiBase.setInterface(b.new Desktop());FModel.initialize(null,null);
        var hp=RegisteredPlayer.forCommander(DeckSerializer.fromFile(Path.of(args[0]).toFile())).setPlayer(new LobbyPlayerHuman("Human"));
        var cp=RegisteredPlayer.forCommander(DeckSerializer.fromFile(Path.of(args[1]).toFile())).setPlayer(GamePlayerUtil.createAiPlayer("CPU",0,"Default"));
        GameRules rules=new GameRules(GameType.Commander);rules.setAppliedVariants(EnumSet.of(GameType.Commander));
        b.game=new Match(rules,List.of(hp,cp),"Choice regression").createGame();b.human=b.game.getPlayers().get(0);
        b.controller=(PlayerControllerHuman)b.human.getController();
        b.gui=(IGuiGame)Proxy.newProxyInstance(IGuiGame.class.getClassLoader(),new Class[]{IGuiGame.class},b);b.controller.setGui(b.gui);
        List<forge.game.replacement.ReplacementEffect> replacements=new ArrayList<>();
        for(String name:List.of("Doubling Season","Hardened Scales")) {
            Card card=make(b,name,ZoneType.Battlefield);
            replacements.add(card.getReplacementEffects().stream().filter(r->"AddCounter".equals(r.getParam("Event"))).findFirst().orElseThrow());
        }
        var replaced=call(()->b.controller.chooseSingleReplacementEffect(replacements));
        var rp=WorkerRegression.await(b);check(rp.min==1&&rp.max==2,"Replacement bounds changed");
        check(b.published.contains("Doubling Season")&&b.published.contains("Hardened Scales"),"Missing replacement source");
        WorkerRegression.reject(b,WorkerRegression.reply(b,rp,"empty-replacement"));
        b.accept(WorkerRegression.reply(b,rp,"replacement-subset",0,1));
        var order=WorkerRegression.await(b);b.accept(WorkerRegression.reply(b,order,"replacement-order",1,0));
        check(replaced.get(2,TimeUnit.SECONDS)==replacements.get(1),"First returned replacement not applied first");
        var again=call(()->b.controller.chooseSingleReplacementEffect(replacements));
        var ap=WorkerRegression.await(b);int chosen=ap.options.indexOf(replacements.get(0).getView());
        b.accept(WorkerRegression.reply(b,ap,"replacement-again",chosen));
        check(again.get(2,TimeUnit.SECONDS)==replacements.get(0),"Remembered replacement was auto-applied");
        WorkerRegression.reject(b,WorkerRegression.reply(b,rp,"old-replacement",0));

        List<forge.game.staticability.StaticAbility> statics=new ArrayList<>();
        for(String name:List.of("Glorious Anthem","Bad Moon"))statics.add(make(b,name,ZoneType.Battlefield).getStaticAbilities().get(0));
        var st=call(()->b.controller.chooseSingleStaticAbility(statics));var sp=WorkerRegression.await(b);
        check(b.published.contains("Glorious Anthem")&&b.published.contains("Bad Moon"),"Static context absent");
        b.accept(WorkerRegression.reply(b,sp,"static",1));check(st.get(2,TimeUnit.SECONDS)==statics.get(1),"Original static object lost");

        List<forge.card.ICardFace> faces=List.of(FModel.getMagicDb().getCommonCards().getFaceByName("Forest"),FModel.getMagicDb().getCommonCards().getFaceByName("Island"));
        var face=call(()->b.controller.chooseSingleCardFace(null,faces,"Name a card"));var fp=WorkerRegression.await(b);
        int island=-1;for(int i=0;i<fp.options.size();i++)if(((CardFaceView)fp.options.get(i)).getName().equals("Island"))island=i;
        b.accept(WorkerRegression.reply(b,fp,"face",island));check(face.get(2,TimeUnit.SECONDS)==faces.get(1),"Catalog face identity lost");
        Card dfc=make(b,"Delver of Secrets",ZoneType.Hand);
        List<CardState> states=List.of(dfc.getState(forge.card.CardStateName.Original),dfc.getState(forge.card.CardStateName.Backside));
        var state=call(()->b.controller.chooseSingleCardState(null,states,"Choose face",Map.of()));var cpending=WorkerRegression.await(b);
        int back=cpending.options.indexOf(states.get(1).getView());
        check(b.published.contains("Insectile Aberration"),"Authorized face context absent");
        b.accept(WorkerRegression.reply(b,cpending,"state",back));check(state.get(2,TimeUnit.SECONDS)==states.get(1),"Original card state lost");
        List<CounterType> counters=List.of(CounterEnumType.P1P1,CounterKeywordType.get("Flying"));
        var counter=call(()->b.controller.chooseCounterType(counters,null,"Choose counter",Map.of()));var kp=WorkerRegression.await(b);
        check(b.published.contains("Flying"),"Keyword counter label absent");b.accept(WorkerRegression.reply(b,kp,"counter",1));check(counter.get(2,TimeUnit.SECONDS)==counters.get(1),"Counter identity lost");

        Card bolt=make(b,"Lightning Bolt",ZoneType.Stack);SpellAbility spell=bolt.getFirstSpellAbility();spell.setActivatingPlayer(b.human);
        var stack=new SpellAbilityStackInstance(spell);
        List<org.apache.commons.lang3.tuple.Pair<SpellAbilityStackInstance,GameObject>> targets=List.of(org.apache.commons.lang3.tuple.Pair.of(stack,b.human),org.apache.commons.lang3.tuple.Pair.of(stack,dfc));
        var target=call(()->b.controller.chooseTarget(spell,targets));var tp=WorkerRegression.await(b);
        check(b.published.contains("Lightning Bolt")&&b.published.contains("Delver of Secrets"),"Redirect target/source absent");
        b.accept(WorkerRegression.reply(b,tp,"target",1));check(target.get(2,TimeUnit.SECONDS)==targets.get(1),"Original redirect pair lost");

        Card hidden=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Doubling Season"),b.game.getPlayers().get(1),b.game);
        b.game.getPlayers().get(1).getZone(ZoneType.Library).add(hidden);
        check(b.label(hidden.getReplacementEffects().get(0).getView()).equals("Hidden ability"),"Hidden replacement disclosed");
        check(b.label(hidden.getCurrentState().getView()).equals("Hidden card face"),"Hidden face disclosed");
        b.reveals.get().add(hidden.getId());check(b.label(hidden.getReplacementEffects().get(0).getView()).contains("Doubling Season"),"Authorized reveal stripped");b.reveals.get().clear();
        check(b.label(hidden.getReplacementEffects().get(0).getView()).equals("Hidden ability"),"Reveal scope persisted");
        check(b.gui.one("Empty",List.of())==null&&b.gui.oneOrNone("Empty",null)==null&&b.pending==null,"Empty list contract changed");
        Files.writeString(out.resolve("checks.json"),ForgeWorker.JSON.toJson(ForgeWorker.map("passed",true,"controllerPaths",List.of("chooseSingleReplacementEffect","chooseSingleStaticAbility","chooseSingleCardFace","chooseSingleCardState","chooseCounterType","chooseTarget"),"originalObjects",true,"hiddenAndScopedReveal",true,"replacementFirstAndNoRemember",true,"browserGameplay",false)));
        System.out.println("PASS typed human choices, original objects, replacement ordering, privacy, empty-list contract");
    }
    static Card make(ForgeWorker b,String name,ZoneType zone) {
        Card c=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard(name),b.human,b.game);if(zone==ZoneType.Stack)b.game.getStackZone().add(c);else b.human.getZone(zone).add(c);return c;
    }
    static <T> CompletableFuture<T> call(Callable<T> action) {
        var f=new CompletableFuture<T>();SwingUtilities.invokeLater(()->{try{f.complete(action.call());}catch(Throwable e){f.completeExceptionally(e);}});return f;
    }
}
