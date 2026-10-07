// Focused real-Forge regression; never drives a measured game or substitutes a controller.
import java.nio.file.*;
import java.util.*;
import java.util.concurrent.*;
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

public final class AbilityProjectionRegression {
    static void check(boolean ok,String message) {if(!ok)throw new AssertionError(message);}
    static JsonObject reply(BrowserSpike b,BrowserSpike.Pending p,String request,int... selected) {
        JsonObject j=new JsonObject();j.addProperty("csrf",b.csrf);j.addProperty("session",b.incarnation);
        j.addProperty("revision",p.revision);j.addProperty("prompt",p.id);j.addProperty("request",request);j.addProperty("action","reply");
        JsonArray ids=new JsonArray();for(int i:selected)ids.add(i);j.add("selected",ids);return j;
    }
    static BrowserSpike.Pending await(BrowserSpike b)throws Exception {
        for(int i=0;i<200;i++){synchronized(b.gate){if(b.pending!=null)return b.pending;}Thread.sleep(10);}
        throw new AssertionError("No prompt");
    }
    static void reject(BrowserSpike b,JsonObject j) {
        try{b.accept(j);throw new AssertionError("Accepted invalid input");}catch(IllegalArgumentException expected){}
    }
    public static void main(String[] args)throws Exception {
        try {run(args);System.exit(0);}catch(Throwable e){e.printStackTrace();System.exit(1);}
    }
    static void run(String[] args)throws Exception {
        Path out=Path.of(args[2]);Files.createDirectories(out);BrowserSpike b=new BrowserSpike(out);
        GuiBase.setInterface(b.new Desktop());FModel.initialize(null,null);
        var hp=RegisteredPlayer.forCommander(DeckSerializer.fromFile(Path.of(args[0]).toFile())).setPlayer(new LobbyPlayerHuman("Human"));
        var cp=RegisteredPlayer.forCommander(DeckSerializer.fromFile(Path.of(args[1]).toFile())).setPlayer(GamePlayerUtil.createAiPlayer("CPU",0,"Default"));
        GameRules rules=new GameRules(GameType.Commander);rules.setAppliedVariants(EnumSet.of(GameType.Commander));
        b.game=new Match(rules,List.of(hp,cp),"Projection regression").createGame();b.human=b.game.getPlayers().get(0);
        Card source=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Arahbo, Roar of the World"),b.human,b.game);
        b.human.getZone(ZoneType.Command).add(source);
        Trigger trigger=source.getTriggers().stream().filter(t->t.hasParam("Secondary")&&"Command".equals(t.getParam("TriggerZones"))).findFirst().orElseThrow();
        check(!trigger.hasParam("TriggerDescription"),"Pinned secondary trigger unexpectedly has prose");
        SpellAbility underlying=trigger.ensureAbility();underlying.setActivatingPlayer(b.human);
        WrappedAbility wrapped=new WrappedAbility(trigger,underlying,b.human);
        wrapped.setTriggeringObject(AbilityKey.Player,b.human);
        SpellAbilityView view=wrapped.getView();String raw=view.getDescription();
        check(raw.equals(wrapped.toUnsuppressedString()),"View lost upstream text");
        check(raw.trim().equals("[Phase: ]"),"Did not reproduce exact upstream description: "+raw);
        String label=b.label(view);
        check(label.contains("Arahbo, Roar of the World")&&label.contains("another target Cat")&&label.contains("+3/+3"),"Missing authorized source/context");
        check(!b.safeText(raw).contains("hidden")&&raw.equals(b.safeText(raw)),"Sanitizer caused missing prose");
        var result=new CompletableFuture<Object>();new Thread(()->{try{result.complete(b.one("getAbilityToPlay","Choose an ability",List.of(view),true));}catch(Exception e){result.completeExceptionally(e);}}).start();
        var p=await(b);check(p.min==0&&p.max==1&&p.options.get(0)==view,"Changed identity or optionality");
        JsonObject stale=reply(b,p,"stale",0);stale.addProperty("revision",p.revision-1);reject(b,stale);
        reject(b,reply(b,p,"invalid",9));check(b.pending==p&&!result.isDone(),"Invalid input consumed prompt");
        ExecutorService pool=Executors.newFixedThreadPool(2);CountDownLatch start=new CountDownLatch(1);
        List<Future<Boolean>> outcomes=new ArrayList<>();
        for(int i=0;i<2;i++){JsonObject j=reply(b,p,"race-"+i,0);outcomes.add(pool.submit(()->{start.await();try{b.accept(j);return true;}catch(IllegalArgumentException e){return false;}}));}
        start.countDown();int accepted=0;for(var f:outcomes)if(f.get())accepted++;
        check(accepted==1&&result.get(2,TimeUnit.SECONDS)==view,"Race or return-value identity failure");
        reject(b,reply(b,p,"race-0",0));reject(b,reply(b,p,"new-replay",0));pool.shutdownNow();
        var optional=new CompletableFuture<Object>();
        new Thread(()->{try{optional.complete(b.one("getAbilityToPlay","Choose an ability",List.of(view),true));}catch(Exception e){optional.completeExceptionally(e);}}).start();
        var op=await(b);b.accept(reply(b,op,"empty"));
        check(optional.get(2,TimeUnit.SECONDS)==null,"Optional empty reply changed");
        try {b.ask("getAbilityToPlay","Ambiguous",List.of(view,view),0,1);throw new AssertionError("Ambiguity accepted");}
        catch(UnsupportedOperationException expected) {check(b.pending==null,"Ambiguous controls published");}
        // Hidden source projection must fail closed; a legitimate explicit reveal is scoped.
        Player cpu=b.game.getPlayers().get(1);
        Card secret=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Benalish Hero"),cpu,b.game);
        cpu.getZone(ZoneType.Library).add(secret);
        check(!b.visible(secret.getView()),"Hidden fixture is visible");
        check(b.label(secret.getView()).equals("Hidden card"),"Hidden source name leaked");
        check(b.label(secret.getFirstSpellAbility().getView()).equals("Hidden ability"),"Hidden ability source/text leaked");
        check(!BrowserSpike.JSON.toJson(b.snapshot("test",null)).contains("Benalish Hero"),"Hidden library identity leaked");
        check(b.safeText("Chosen Benalish Hero").equals("Chosen [hidden card]"),"Hidden prose identity leaked");
        var revealDone=new CompletableFuture<String>();
        var method=Arrays.stream(IGuiGame.class.getMethods()).filter(m->m.getName().equals("reveal")).findFirst().orElseThrow();
        new Thread(()->{try{b.invoke(null,method,new Object[]{null,List.of(secret.getView())});revealDone.complete(b.label(secret.getView()));}catch(Throwable e){revealDone.completeExceptionally(e);}}).start();
        var rp=await(b);check(b.published.contains("Benalish Hero"),"Legitimate reveal censored");b.accept(reply(b,rp,"reveal"));
        check(revealDone.get(2,TimeUnit.SECONDS).equals("Hidden card"),"Reveal authorization persisted");
        check(!BrowserSpike.JSON.toJson(b.snapshot("test",null)).contains("Benalish Hero"),"Reveal leaked after scope");
        Files.writeString(out.resolve("checks.json"),BrowserSpike.JSON.toJson(BrowserSpike.map("passed",true,"upstreamDescription",raw,"label",label,"optionIdentity",true,"optional",true,"competingRepliesAccepted",accepted,"hiddenIdentityExcluded",true,"scopedReveal",true)));
        System.out.println("PASS actual secondary-trigger view, source context, identity, optionality, invalid/stale/repeated/competing replies, hidden library and scoped reveal");
    }
}
