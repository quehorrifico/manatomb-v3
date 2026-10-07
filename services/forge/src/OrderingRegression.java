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

public final class OrderingRegression {
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
        b.game=new Match(rules,List.of(hp,cp),"Ordering regression").createGame();b.human=b.game.getPlayers().get(0);
        b.controller=(PlayerControllerHuman)b.human.getController();
        b.gui=(IGuiGame)Proxy.newProxyInstance(IGuiGame.class.getClassLoader(),new Class[]{IGuiGame.class},b);b.controller.setGui(b.gui);
        List<SpellAbility> abilities=new ArrayList<>();
        Card attacker=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Leonin Shikari"),b.human,b.game);b.human.getZone(ZoneType.Battlefield).add(attacker);
        for(String name:List.of("Arahbo, Roar of the World","Sword of the Animist")) {
            Card card=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard(name),b.human,b.game);b.human.getZone(ZoneType.Battlefield).add(card);
            Trigger t=card.getTriggers().stream().filter(x->"Attacks".equals(x.getParam("Mode"))).findFirst().orElseThrow();
            SpellAbility inner=t.ensureAbility();inner.setActivatingPlayer(b.human);
            WrappedAbility wrapped=new WrappedAbility(t,inner,b.human);wrapped.setTriggeringObject(AbilityKey.Card,attacker);abilities.add(wrapped);
        }
        var future=new CompletableFuture<List<SpellAbility>>();
        SwingUtilities.invokeLater(()->{try{future.complete(b.controller.orderSimultaneousSa(abilities));}catch(Throwable e){future.completeExceptionally(e);}});
        var pending=WorkerRegression.await(b);
        check(pending.kind.equals("orderSimultaneousAbilities")&&pending.min==2&&pending.max==2,"Missing permutation contract");
        check(pending.options.get(0)==abilities.get(0).getView()&&pending.options.get(1)==abilities.get(1).getView(),"Offered view identity changed");
        int rejected=0;
        for(String entries:List.of("[]","[0]","[0,0]","[0,1,2]","[0,2]","[-1,1]","[0.5,1]","[\"0\",1]","[null,1]","[4294967296,1]")) {
            JsonObject reply=WorkerRegression.reply(b,pending,"bad-"+rejected);reply.add("selected",JsonParser.parseString(entries));WorkerRegression.reject(b,reply);rejected++;
        }
        JsonObject stale=WorkerRegression.reply(b,pending,"stale",1,0);stale.addProperty("revision",pending.revision-1);WorkerRegression.reject(b,stale);
        JsonObject remember=WorkerRegression.reply(b,pending,"remember",1,0);remember.addProperty("rememberDecision",true);WorkerRegression.reject(b,remember);
        check(!future.isDone()&&b.pending==pending,"Bad order consumed/unblocked the prompt");
        var start=new CountDownLatch(1);var pool=Executors.newFixedThreadPool(2);List<Future<Boolean>> races=new ArrayList<>();
        for(int i=0;i<2;i++){var reply=WorkerRegression.reply(b,pending,"race-"+i,1,0);races.add(pool.submit(()->{start.await();try{b.accept(reply);return true;}catch(IllegalArgumentException e){return false;}}));}
        start.countDown();int accepted=0;for(var f:races)if(f.get())accepted++;pool.shutdownNow();
        var returned=future.get(2,TimeUnit.SECONDS);check(accepted==1&&returned.get(0)==abilities.get(1)&&returned.get(1)==abilities.get(0),"Competing replies or original ability mapping failed");

        WorkerRegression.reject(b,WorkerRegression.reply(b,pending,"replay",1,0));
        // Forge caches a proposed order even with remember=false. It MUST ask again.
        var again=new CompletableFuture<List<SpellAbility>>();SwingUtilities.invokeLater(()->{try{again.complete(b.controller.orderSimultaneousSa(abilities));}catch(Throwable e){again.completeExceptionally(e);}});
        var second=WorkerRegression.await(b);check(!again.isDone()&&second.id!=pending.id,"Decision silently remembered");
        List<SpellAbilityView> secondViews=new ArrayList<>();for(Object v:second.options)secondViews.add((SpellAbilityView)v);
        int a=secondViews.indexOf(abilities.get(0).getView()),z=secondViews.indexOf(abilities.get(1).getView());
        b.accept(WorkerRegression.reply(b,second,"second",a,z));check(again.get(2,TimeUnit.SECONDS).get(0)==abilities.get(0),"Explicit changed order ignored");
        var general=new CompletableFuture<IGuiGame.OrderResult<CardView>>();SwingUtilities.invokeLater(()->{try{general.complete(b.gui.order("General card order","First",0,0,List.of(attacker.getView()),null,null,false,false));}catch(Throwable e){general.completeExceptionally(e);}});
        var gp=WorkerRegression.await(b);b.accept(WorkerRegression.reply(b,gp,"explicit-card-order",0));check(general.get(2,TimeUnit.SECONDS).ordered().get(0)==attacker.getView(),"General card view identity lost");
        List<CardView> subsetOptions=List.of(attacker.getView(),abilities.get(0).getHostCard().getView(),abilities.get(1).getHostCard().getView());
        var subset=new CompletableFuture<IGuiGame.OrderResult<CardView>>();SwingUtilities.invokeLater(()->{try{subset.complete(b.gui.order("Choose cards for destination","Top first",1,2,subsetOptions,null,null,false,false));}catch(Throwable e){subset.completeExceptionally(e);}});
        var sp=WorkerRegression.await(b);check(sp.min==1&&sp.max==2,"Wrong subset bounds");b.accept(WorkerRegression.reply(b,sp,"explicit-subset",0,2));
        var arranged=WorkerRegression.await(b);check(arranged.kind.equals("orderCards")&&!subset.isDone(),"Subset silently returned in source order");
        b.accept(WorkerRegression.reply(b,arranged,"explicit-subset-order",1,0));
        var subsetResult=subset.get(2,TimeUnit.SECONDS).ordered();check(subsetResult.get(0)==subsetOptions.get(2)&&subsetResult.get(1)==subsetOptions.get(0),"Explicit subset ordering lost original view identity");
        Files.writeString(out.resolve("checks.json"),ForgeWorker.JSON.toJson(ForgeWorker.map("passed",true,"actualController","PlayerControllerHuman.orderSimultaneousSa","thread","AWT-EventQueue-0","invalidPermutationsRejected",rejected,"competingRepliesAccepted",accepted,"originalViewAndAbilityIdentity",true,"rememberDisabledAndReprompted",true,"generalCardOrderingOriginalView",true,"stackResolution","Verified separately by focused real-browser fixture")));
        System.out.println("PASS actual controller original-view mapping, explicit permutation validation, stale/replay/race, blocked EDT safe reply, no remember, unsupported modes");
    }
}
