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

public final class WorkerRegression {
    static void check(boolean ok,String message) {if(!ok)throw new AssertionError(message);}
    static JsonObject reply(ForgeWorker b,ForgeWorker.Pending p,String request,int... selected) {
        JsonObject j=new JsonObject();j.addProperty("csrf",b.csrf);j.addProperty("session",b.incarnation);
        j.addProperty("revision",p.revision);j.addProperty("prompt",p.id);j.addProperty("request",request);j.addProperty("action","reply");
        JsonArray ids=new JsonArray();for(int i:selected)ids.add(i);j.add("selected",ids);return j;
    }
    static ForgeWorker.Pending await(ForgeWorker b)throws Exception {
        for(int i=0;i<200;i++){synchronized(b.gate){if(b.pending!=null)return b.pending;}Thread.sleep(10);}
        throw new AssertionError("No prompt");
    }
    static void reject(ForgeWorker b,JsonObject j) {
        try{b.accept(j);throw new AssertionError("Accepted invalid input");}catch(IllegalArgumentException expected){}
    }
    public static void main(String[] args)throws Exception {
        try {run(args);System.exit(0);}catch(Throwable e){e.printStackTrace();System.exit(1);}
    }
    static void run(String[] args)throws Exception {
        Path out=Path.of(args[2]);Files.createDirectories(out);ForgeWorker b=new ForgeWorker(out);
        GuiBase.setInterface(b.new Desktop());FModel.initialize(null,null);
        var hp=RegisteredPlayer.forCommander(DeckSerializer.fromFile(Path.of(args[0]).toFile())).setPlayer(new LobbyPlayerHuman("Human"));
        var cp=RegisteredPlayer.forCommander(DeckSerializer.fromFile(Path.of(args[1]).toFile())).setPlayer(GamePlayerUtil.createAiPlayer("CPU",0,"Default"));
        GameRules rules=new GameRules(GameType.Commander);rules.setAppliedVariants(EnumSet.of(GameType.Commander));
        b.game=new Match(rules,List.of(hp,cp),"Projection regression").createGame();b.human=b.game.getPlayers().get(0);
        Card source=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Arahbo, Roar of the World"),b.human,b.game);
        b.human.getZone(ZoneType.Command).add(source);
        Card internal=new forge.game.ability.effects.DetachedCardEffect(b.human,"Commander Effect");
        b.human.getZone(ZoneType.Command).add(internal);
        Card keywordBookkeeping=b.human.getKeywordCard();
        check(keywordBookkeeping.getRenderForUI(),"Did not reproduce renderable Keyword Effects bookkeeping");
        JsonObject projection=JsonParser.parseString(ForgeWorker.JSON.toJson(b.snapshot("test",null))).getAsJsonObject();
        JsonObject command=null;
        for(JsonElement z:projection.getAsJsonArray("players").get(0).getAsJsonObject().getAsJsonArray("zones"))if(z.getAsJsonObject().get("zone").getAsString().equals("Command"))command=z.getAsJsonObject();
        check(command!=null&&command.get("count").getAsInt()==1&&command.getAsJsonArray("cards").size()==1,"Internal commander effect inflated command zone");
        check(!projection.toString().contains("Commander Effect")&&!projection.toString().contains("Keyword Effects")&&projection.toString().contains("Arahbo, Roar of the World"),"Hidden bookkeeping exported or real commander lost");
        Card emblem=new Card(b.game.nextCardId(),null,b.game);emblem.setOwner(b.human);emblem.setName("Keyword Effects");emblem.setGamePieceType(forge.card.GamePieceType.EFFECT);emblem.setEmblem(true);emblem.setImageKey(forge.ImageKeys.HIDDEN_CARD);
        check(b.displayCard(emblem,ZoneType.Command),"Name filter hid a real emblem");
        check(b.displayCard(keywordBookkeeping,ZoneType.Battlefield),"Bookkeeping filter escaped its command-zone scope");
        // Real token script names include "Token"; Scryfall's identity does not.
        var eggPaper=forge.StaticData.instance().getAllTokens().getToken("g_0_1_egg_defender","C19");
        check(eggPaper!=null,"Pinned Egg token missing");
        Card egg=CardFactory.getCard(eggPaper,b.human,b.game);b.human.getZone(ZoneType.Battlefield).add(egg);
        Map<String,Object> eggView=b.card(egg.getView(),null);
        check(Boolean.TRUE.equals(eggView.get("token")) && "Egg".equals(eggView.get("tokenName")) && "0/1".equals(eggView.get("tokenPT")),"Egg token artwork identity not projected: "+eggView);
        check(ForgeWorker.JSON.toJson(b.snapshot("test",null)).contains("Egg Token"),"Real Egg token missing from battlefield");
        String print="4e5c0438-e691-4c5f-9d04-1861f9588a09";
        Path printed=out.resolve("printing-deck.json");Files.writeString(printed,ForgeWorker.JSON.toJson(ForgeWorker.map("name","Printing fixture","commanders",List.of(ForgeWorker.map("name","Arahbo, Roar of the World","quantity",1,"printing",print)),"main",List.of(ForgeWorker.map("name","Forest","quantity",99)))));
        Deck printedDeck=b.readDeck(printed);b.playerPrintings.put(b.human.getId(),b.deckPrintings.get(printedDeck));
        check(print.equals(b.card(source.getView(),null).get("printing")),"Saved commander printing lost");
        Card printedLand=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Forest"),b.human,b.game);
        b.playerPrintings.get(b.human.getId()).put(printedLand.getPaperCard().getName(),print);
        b.human.getZone(ZoneType.Library).add(printedLand);
        check(!b.card(printedLand.getView(),null).containsKey("printing"),"Hidden printing identity leaked");
        check(b.focusZones(List.of(printedLand.getView()),true).isEmpty(),"Hidden library auto-focus leaked");
        b.reveals.get().add(printedLand.getId());
        check(b.focusZones(List.of(printedLand.getView()),true).equals(List.of(ForgeWorker.map("player",b.human.getId(),"zone","Library"))),"Authorized search did not focus library");
        check(print.equals(b.card(printedLand.getView(),null).get("printing")),"Revealed saved printing lost");b.reveals.get().clear();
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
        reject(b,reply(b,p,"new-replay",0));pool.shutdownNow();
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
        check(!ForgeWorker.JSON.toJson(b.snapshot("test",null)).contains("Benalish Hero"),"Hidden library identity leaked");
        check(b.permittedOrder(List.of(secret.getView(),secret.getView())).equals("2 hidden cards"),"Hidden order not compact/anonymous");
        check(b.safeText("Chosen Benalish Hero").equals("Chosen [hidden card]"),"Hidden prose identity leaked");
        check(b.safeText("Chosen Benalish Hero ("+secret.getId()+")").equals("Chosen [hidden card]"),"Hidden raw entity reference leaked");
        var revealDone=new CompletableFuture<String>();
        var method=Arrays.stream(IGuiGame.class.getMethods()).filter(m->m.getName().equals("reveal")).findFirst().orElseThrow();
        new Thread(()->{try{b.invoke(null,method,new Object[]{"Cards revealed by the game",List.of(secret.getView())});revealDone.complete(b.label(secret.getView()));}catch(Throwable e){revealDone.completeExceptionally(e);}}).start();
        var rp=await(b);check(b.published.contains("Cards revealed by the game"),"Reveal explanation was lost");check(b.published.contains("Benalish Hero"),"Legitimate reveal censored");
        JsonObject revealView=JsonParser.parseString(b.published).getAsJsonObject();
        boolean temporaryFound=false;
        for(JsonElement player:revealView.getAsJsonArray("players"))for(JsonElement zone:player.getAsJsonObject().getAsJsonArray("zones"))for(JsonElement card:zone.getAsJsonObject().getAsJsonArray("cards")) {
            JsonObject facts=card.getAsJsonObject();
            if(facts.get("name").getAsString().equals("Benalish Hero")) {
                check(facts.get("temporaryReveal").getAsBoolean(),"Scoped reveal missing expiry metadata");temporaryFound=true;
            }
        }
        check(temporaryFound,"Revealed card absent from snapshot");
        check(Boolean.FALSE.equals(b.card(source.getView(),null).get("temporaryReveal")),"Public commander marked temporary");
        b.reveals.get().add(source.getId());
        check(Boolean.FALSE.equals(b.card(source.getView(),null).get("temporaryReveal")),"Public card in reveal scope marked temporary");
        b.reveals.get().clear();
        b.accept(reply(b,rp,"reveal"));
        check(revealDone.get(2,TimeUnit.SECONDS).equals("Hidden card"),"Reveal authorization persisted");
        check(!ForgeWorker.JSON.toJson(b.snapshot("test",null)).contains("Benalish Hero"),"Reveal leaked after scope");
        check(!b.card(secret.getView(),null).containsKey("temporaryReveal"),"Hidden card exposes reveal metadata");
        Card tiger=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Seht's Tiger"),b.human,b.game);
        b.human.getZone(ZoneType.Battlefield).add(tiger);
        Trigger enters=tiger.getTriggers().get(0);SpellAbility entersAbility=enters.ensureAbility();entersAbility.setActivatingPlayer(b.human);
        WrappedAbility entersWrapped=new WrappedAbility(enters,entersAbility,b.human);entersWrapped.setTriggeringObject(AbilityKey.Card,tiger);
        String entersRaw=entersWrapped.getView().getDescription();
        check(entersRaw.contains("Zone Changer:"),"Did not reproduce localized stack annotation");
        check(!b.label(entersWrapped.getView()).contains("Zone Changer:")&&!b.label(entersWrapped.getView()).contains("("+tiger.getId()+")"),"Engine annotation leaked from ability label");
        check(ForgeWorker.withoutStackMetadata(entersRaw).contains("protection"),"Authorized ability prose removed");
        Card effect=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Mosswort Bridge"),b.human,b.game);
        effect.setName("Mosswort Bridge (195)'s Effect");b.human.getZone(ZoneType.Command).add(effect);
        check(b.cardName(effect.getView()).equals("Mosswort Bridge's Effect"),"Generated effect exposes source engine ID");
        b.snapshot("test",null);
        String permittedRef=b.safeText("Block Seht's Tiger ("+tiger.getId()+")");
        check(permittedRef.contains("[Card "+b.publicIds.get(tiger.getId())+"]")&&!permittedRef.contains("("+tiger.getId()+")"),"Blocker prompt reference not translated to current projection token");
        // A hidden basic land must not erase an identical public land-type clause.
        Card grasslands=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Grasslands"),b.human,b.game);b.human.getZone(ZoneType.Battlefield).add(grasslands);
        Card hiddenPlains=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Plains"),cpu,b.game);cpu.getZone(ZoneType.Library).add(hiddenPlains);
        String publicRule="Search your library for a Forest or Plains card, put it onto the battlefield, then shuffle.";
        check(b.safeText(publicRule).equals(publicRule),"Public Oracle clause incorrectly censored by hidden card name");
        check(b.safeText("Private selected card: Plains").equals("Private selected card: [hidden card]"),"Public rule globally authorized a hidden identity");
        check(b.safeText(publicRule+" Private selected card: Plains").equals(publicRule+" Private selected card: [hidden card]"),"Hidden name outside public clause leaked");
        check(b.label(grasslands.getSpellAbilities().stream().filter(a->a.hasParam("SpellDescription") && a.getParam("SpellDescription").contains("Forest or Plains")).findFirst().orElseThrow().getView()).contains("Forest or Plains"),"Actual Grasslands ability label still ambiguous");
        b.snapshot("test",null);Map<Integer,Integer> oldReferences=new HashMap<>(b.publicIds);String oldLabel=b.label(grasslands.getView());
        b.snapshot("test",null);String relabeled=b.refreshReferences(oldLabel,oldReferences);
        check(relabeled.equals(b.label(grasslands.getView())),"Prompt source reference did not follow the fresh projection");
        b.publicIds.remove(grasslands.getId());check(!b.refreshReferences(oldLabel,oldReferences).matches(".*\\[Card [0-9]+\\].*"),"Retired reference survived a hidden transition");
        // Contract deck inputs are immutable catalog names, including paired commanders.
        for(String[] commanders:List.of(new String[]{"Tymna the Weaver","Thrasios, Triton Hero"},new String[]{"Wilson, Refined Grizzly","Raised by Giants"})) {
            List<Object> rows=new ArrayList<>();for(String name:commanders)rows.add(ForgeWorker.map("name",name,"quantity",1));
            Path input=out.resolve("paired-deck.json");Files.writeString(input,ForgeWorker.JSON.toJson(ForgeWorker.map("name","Paired fixture","commanders",rows,"main",List.of(ForgeWorker.map("name",commanders[0].startsWith("Wilson")?"Forest":"Plains","quantity",98)))));
            Deck paired=b.readDeck(input);check(DeckFormat.Commander.getDeckConformanceProblem(paired)==null,"Pinned Forge rejected paired commander fixture");check(paired.get(DeckSection.Commander).countAll()==2,"Lost commander section");
        }
        Path unknown=out.resolve("unknown-deck.json");Files.writeString(unknown,"{\"name\":\"Unknown\",\"commanders\":[{\"name\":\"Not a Forge card 123456\",\"quantity\":1}],\"main\":[]}");
        try{b.readDeck(unknown);throw new AssertionError("Unknown card silently dropped");}catch(IllegalArgumentException expected){}
        // Real catalog views exercise the extracted Forge combat assignment constraints.
        Card trampler=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Colossal Dreadmaw"),b.human,b.game);
        Card bear=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Grizzly Bears"),cpu,b.game);
        Card spider=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Giant Spider"),cpu,b.game);
        List<CardView> targets=PinnedCombatAssignment.targets(trampler.getView(),List.of(bear.getView(),spider.getView()),cpu.getView(),false);
        check(targets.size()==3,"Missing trample defender");
        PinnedCombatAssignment.validate(trampler.getView(),targets,cpu.getView(),false,6,10,List.of(2,4,0));
        try{PinnedCombatAssignment.validate(trampler.getView(),targets,cpu.getView(),false,6,10,List.of(0,0,6));throw new AssertionError("Illegal trample accepted");}catch(IllegalArgumentException expected){}
        try{PinnedCombatAssignment.validate(trampler.getView(),targets,cpu.getView(),false,6,10,List.of(2,4,1));throw new AssertionError("Invented damage accepted");}catch(IllegalArgumentException expected){}
        // A reply arriving on a separate thread unblocks a synchronous scalar prompt.
        var integer=new CompletableFuture<List<Integer>>();javax.swing.SwingUtilities.invokeLater(()->{try{integer.complete(b.askValues("integer-test","Choose X",List.of(),0,0,"integer",values->{if(values.get(0)<0||values.get(0)>1000)throw new IllegalArgumentException("range");},Map.of("lower",0,"upper",1000)));}catch(Exception e){integer.completeExceptionally(e);}});
        var ip=await(b);JsonObject bad=reply(b,ip,"invalid-integer");bad.addProperty("value",0.5);reject(b,bad);check(!integer.isDone(),"Malformed scalar consumed input");
        JsonObject good=reply(b,ip,"integer-reply");good.addProperty("value",200);b.accept(good);check(integer.get(2,TimeUnit.SECONDS).equals(List.of(200)),"scalar reply failed");
        check(b.accept(good).contains("true"),"Idempotent receipt failed");good.addProperty("value",201);reject(b,good);
        var visibleCounters=com.google.common.collect.HashMultiset.<CounterType>create();visibleCounters.add(CounterEnumType.P1P1,2);tiger.setCounters(visibleCounters);tiger.setDamage(1);
        String publicFacts=ForgeWorker.JSON.toJson(b.card(tiger.getView(),null));
        check(publicFacts.contains("\"damage\":1")&&publicFacts.contains("\"count\":2"),"Public damage/counters absent");
        check(Boolean.TRUE.equals(b.card(tiger.getView(),null).get("statsModified")),"Counter-modified PT not marked");
        check(b.card(tiger.getView(),null).get("pt").equals(tiger.getNetPower()+"/"+tiger.getNetToughness()),"Displayed PT differs from Forge");
        b.human.setCounters(CounterEnumType.POISON,3,null,false);b.human.setCounters(CounterEnumType.ENERGY,2,null,false);b.human.setCounters(CounterEnumType.EXPERIENCE,5,null,false);
        var playerCounters=JsonParser.parseString(ForgeWorker.JSON.toJson(b.snapshot("test",null))).getAsJsonObject().getAsJsonArray("players").get(0).getAsJsonObject().getAsJsonObject("counters");
        check(playerCounters.get("poison").getAsInt()==3&&playerCounters.get("energy").getAsInt()==2&&playerCounters.get("experience").getAsInt()==5,"Player counters not projected");
        b.event(new forge.game.event.GameEventLandPlayed(b.human.getView(),egg.getView()));
        b.event(new forge.game.event.GameEventLandPlayed(cpu.getView(),secret.getView()));
        check(ForgeWorker.JSON.toJson(b.publicLog).contains("Egg Token")&&!ForgeWorker.JSON.toJson(b.publicLog).contains("Benalish Hero"),"Typed event log exposed a hidden identity");
        // A normal land play emits a zone event first. Replace precisely that
        // arrival, while retaining arrivals caused by library/tokens/effects.
        Card mountain=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Mountain"),b.human,b.game);
        b.human.getZone(ZoneType.Battlefield).add(mountain);
        int logStart=b.publicLog.size();
        b.event(new forge.game.event.GameEventCardChangeZone(mountain,b.human.getZone(ZoneType.Hand),b.human.getZone(ZoneType.Battlefield)));
        b.event(new forge.game.event.GameEventCardChangeZone(egg,null,b.human.getZone(ZoneType.Battlefield)));
        b.event(new forge.game.event.GameEventLandPlayed(b.human.getView(),mountain.getView()));
        String landLog=ForgeWorker.JSON.toJson(b.publicLog.subList(logStart,b.publicLog.size()));
        check(b.publicLog.size()==logStart+2 && landLog.contains("You played Mountain") && !landLog.contains("Mountain entered") && landLog.contains("Egg Token entered"),"Land log duplication or meaningful arrival lost: "+landLog);
        b.event(new forge.game.event.GameEventCardChangeZone(tiger.getView(),new forge.game.zone.ZoneView(b.human.getView(),ZoneType.Stack),b.human.getZone(ZoneType.Battlefield).getView()));
        b.event(new forge.game.event.GameEventSpellResolved(tiger.getFirstSpellAbility(),false));
        check(b.publicLog.size()==logStart+2,"Successful permanent repeated cast/arrival/resolution");
        b.event(new forge.game.event.GameEventSpellResolved(tiger.getFirstSpellAbility(),true));
        check(b.publicLog.size()==logStart+3 && ForgeWorker.JSON.toJson(b.publicLog).contains("did not resolve"),"Failed resolution was hidden");
        // Target arrows contain current public projection IDs only. An obsolete
        // public ID must never recover a card now concealed in a library.
        Card bolt=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Lightning Bolt"),b.human,b.game);
        SpellAbility targeted=bolt.getFirstSpellAbility();targeted.setActivatingPlayer(b.human);
        targeted.getTargets().add(tiger);targeted.getTargets().add(secret);targeted.getTargets().add(cpu);
        b.snapshot("test",null);b.publicIds.put(secret.getId(),999999);
        var projectedTargets=b.stackTargets(new SpellAbilityStackInstance(targeted).getView());
        check(projectedTargets.size()==2 && projectedTargets.contains(ForgeWorker.map("type","card","id",b.publicIds.get(tiger.getId()))) && projectedTargets.contains(ForgeWorker.map("type","player","id",cpu.getId())),"Public card/player targets missing: "+projectedTargets);
        check(!ForgeWorker.JSON.toJson(projectedTargets).contains("999999"),"Concealed target exposed a prior reference");b.publicIds.remove(secret.getId());
        String zoneMarker=" ["+forge.util.Localizer.getInstance().getMessage("lblZoneChanger")+": ";
        check(b.publicZoneContext(zoneMarker+secret+"]").isEmpty(),"Hidden zone-change context leaked");
        // Force a prior public projection, then conceal the entity. Neither its
        // old engine string nor its old cache entry may recover that identity.
        cpu.getZone(ZoneType.Library).remove(secret);cpu.getZone(ZoneType.Battlefield).add(secret);b.snapshot("test",null);String oldSecret=secret.toString();
        cpu.getZone(ZoneType.Battlefield).remove(secret);cpu.getZone(ZoneType.Library).add(secret);b.snapshot("test",null);
        check(b.publicZoneContext(zoneMarker+oldSecret+"]").isEmpty(),"Public history exposed a now-hidden card");
        var skip=new CompletableFuture<Object>();javax.swing.SwingUtilities.invokeLater(()->{try{skip.complete(b.assignCombat(new Object[]{trampler.getView(),List.of(bear.getView()),6,cpu.getView(),false,true}));}catch(Exception e){skip.completeExceptionally(e);}});
        var sk=await(b);check(sk.kind.equals("skipCombatAssignment"),"Optional combat skip not offered");b.accept(reply(b,sk,"skip-assignment",1));check(skip.get(2,TimeUnit.SECONDS)==null,"Skip differs from pinned GUI null result");
        Map<Object,Integer> allocations=new LinkedHashMap<>();allocations.put(b.human.getView(),5);allocations.put(cpu.getView(),7);
        var divided=new CompletableFuture<Object>();javax.swing.SwingUtilities.invokeLater(()->{try{divided.complete(b.assignAmounts(new Object[]{null,allocations,3,false,"Divide three"}));}catch(Exception e){divided.completeExceptionally(e);}});
        var dp=await(b);var badAmount=reply(b,dp,"bad-amount");badAmount.add("amounts",JsonParser.parseString("[4,-1]"));reject(b,badAmount);check(!divided.isDone(),"Invalid distribution consumed prompt");
        var amountReply=reply(b,dp,"amounts");amountReply.add("amounts",JsonParser.parseString("[1,2]"));b.accept(amountReply);
        Map<?,?> distribution=(Map<?,?>)divided.get(2,TimeUnit.SECONDS);check(distribution.get(b.human.getView()).equals(1)&&distribution.get(cpu.getView()).equals(2),"Distribution lost original target identity");
        cpu.getZone(ZoneType.Library).remove(secret);cpu.getZone(ZoneType.Battlefield).add(secret);b.snapshot("test",null);String faceUpReference=secret.toString();secret.turnFaceDown(true);b.snapshot("test",null);
        check(b.publicZoneContext(zoneMarker+faceUpReference+"]").isEmpty(),"Public history recovered a face-down identity");
        var facePending=new ForgeWorker.Pending();String facedown=ForgeWorker.JSON.toJson(b.snapshot("test",facePending));
        check(!facedown.contains("Benalish Hero")&&facedown.contains("Face-down card"),"Face-down battlefield identity leaked");
        check(facePending.cards.containsValue(secret.getView()),"Public face-down permanent cannot be targeted");
        check(!b.card(secret.getView(),facePending).containsKey("rules"),"Hidden printed text exposed");
        // Real engine views: copied token and both faces project their current
        // public characteristics, without a synthetic browser-side card model.
        Card delver=CardFactory.getCard(FModel.getMagicDb().getCommonCards().getCard("Delver of Secrets"),b.human,b.game);b.human.getZone(ZoneType.Battlefield).add(delver);
        check(b.card(delver.getView(),null).get("name").equals("Delver of Secrets"),"Front face absent");
        delver.setBackSide(true);check(delver.changeToState(forge.card.CardStateName.Backside),"Actual back face unavailable");
        var back=b.card(delver.getView(),null);
        check(back.get("name").equals("Insectile Aberration")&&back.get("pt").equals("3/2")&&Boolean.TRUE.equals(back.get("backFace")),"Transformed current characteristics lost");
        Card copy=CardCopyService.copyStats(delver,b.human,true);copy.setGamePieceType(forge.card.GamePieceType.TOKEN);b.human.getZone(ZoneType.Battlefield).add(copy);copy.updateStateForView();
        var token=b.card(copy.getView(),null);
        check(Boolean.TRUE.equals(token.get("token"))&&token.get("pt").equals(back.get("pt"))&&token.get("name").equals(back.get("name")),"Copied token view differs from engine");
        check(!token.get("id").equals(back.get("id")),"Copy shared projected identity with source");
        check(!b.card(secret.getView(),null).containsKey("backFace")&&!b.card(secret.getView(),null).containsKey("copy"),"Unauthorized face metadata exposed");
        Files.writeString(out.resolve("checks.json"),ForgeWorker.JSON.toJson(ForgeWorker.map("passed",true,"upstreamDescription",raw,"label",label,"optionIdentity",true,"optional",true,"competingRepliesAccepted",accepted,"hiddenIdentityExcluded",true,"scopedReveal",true)));
        System.out.println("PASS pinned ability projection, privacy/reveal, competing/stale replies, receipt, blocked EDT scalar and combat constraints");
    }
}
