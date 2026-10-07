// Production adapter derived from the verified spike; pinned Forge owns all gameplay.
// Historical spike and its evidence are intentionally retained unchanged.
import com.google.gson.*;
import com.google.common.eventbus.Subscribe;
import com.sun.net.httpserver.*;
import forge.GuiDesktop;
import forge.gui.GuiBase;
import forge.gui.interfaces.IGuiGame;
import forge.model.FModel;
import forge.deck.*;
import forge.deck.io.DeckSerializer;
import forge.game.*;
import forge.game.card.*;
import forge.game.player.*;
import forge.game.spellability.*;
import forge.game.replacement.ReplacementEffectView;
import forge.game.staticability.StaticAbility;
import forge.game.staticability.StaticAbilityView;
import org.apache.commons.lang3.tuple.Pair;
import forge.game.zone.ZoneType;
import forge.game.phase.PhaseType;
import forge.gamemodes.match.YieldUpdate;
import forge.game.event.*;
import forge.player.*;
import forge.gamemodes.match.input.*;
import forge.util.MyRandom;
import javax.swing.SwingUtilities;
import java.lang.reflect.*;
import java.net.*;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.io.*;
import java.util.*;
import java.util.concurrent.*;

public final class ForgeWorker implements InvocationHandler {
    static final Gson JSON = new GsonBuilder().disableHtmlEscaping().create();
    final Object gate = new Object();
    final String incarnation = UUID.randomUUID().toString();
    final String csrf = Objects.requireNonNull(System.getenv("FORGE_WORKER_TOKEN"), "worker token required");
    final LinkedHashMap<String,String> consumed = new LinkedHashMap<>();
    final Map<Integer,Integer> publicIds=new HashMap<>();
    final Map<String,Set<PhaseType>> skippedPhases=new LinkedHashMap<>(Map.of("Human",new HashSet<>(),"CPU",new HashSet<>()));
    int nextPublicId=1;
    final Set<Integer> selectable = new HashSet<>(), highlighted = new HashSet<>();
    final List<Map<String,Object>> trace = new ArrayList<>();
    final Path output;
    final ThreadLocal<Set<Integer>> reveals=ThreadLocal.withInitial(HashSet::new);
    final ThreadLocal<Boolean> revealText=ThreadLocal.withInitial(()->false);
    final ThreadLocal<Integer> clickedCard=new ThreadLocal<>();
    volatile String published = "{\"status\":\"starting\",\"revision\":0}";
    Game game; Player human; PlayerControllerHuman controller; IGuiGame gui;
    long revision, paintSerial; Pending pending; Input shownInput, paintedInput;
    String promptText="", okLabel="OK", cancelLabel="Cancel";
    boolean okEnabled, cancelEnabled, invalidAction, actionInFlight;
    boolean concessionAvailable, concessionRequested, gameFinished;
    int port; boolean focused, blockingFixture, orderingFixture, choicesFixture, verificationFixture; volatile boolean blocked;
    String actionError="";
    static final class Pending {
        String id=UUID.randomUUID().toString(), kind;
        long revision; Input input; List<?> options; int min,max;
        CompletableFuture<List<Integer>> reply;
        Map<Integer,CardView> cards=new HashMap<>();
        Map<Integer,PlayerView> players=new HashMap<>();
        boolean ok,cancel,concede;
        Set<Integer> mana=new HashSet<>();
        String valueType="selection"; String textReply;
        java.util.function.Consumer<List<Integer>> validator;
    }
    ForgeWorker(Path output) { this.output=output; }
    static Map<String,Object> map(Object... pairs) {
        Map<String,Object> result=new LinkedHashMap<>();
        for(int i=0;i<pairs.length;i+=2) result.put((String)pairs[i],pairs[i+1]);
        return result;
    }
    static String failureReference(Throwable failure) {
        String message=Objects.toString(failure.getMessage(),"");
        if(message.matches("Unmapped GUI method: [A-Za-z]+"))return "unsupported_gui_"+message.substring("Unmapped GUI method: ".length());
        if(message.matches("Unmapped choice type [A-Za-z0-9_.$]+"))return "unsupported_choice_"+message.substring("Unmapped choice type ".length());
        String operation="engine";
        for(StackTraceElement frame:failure.getStackTrace())if(frame.getClassName().equals("ForgeWorker") && Set.of("snapshot","card","label","abilityLabel","askValues","assignCombat","assignAmounts","orderSimultaneous","publishInput","accept","invoke").contains(frame.getMethodName())){operation=frame.getMethodName();break;}
        String type=failure.getClass().getSimpleName();if(!type.matches("[A-Za-z0-9_]{1,60}"))type="Exception";
        return "adapter_failure_"+operation+"_"+type;
    }
    void fail(Throwable failure) {
        // Stack frames identify the adapter seam without recording exception messages,
        // card identities, raw game objects, or private deck contents.
        System.err.println("Forge adapter failure: "+failure.getClass().getSimpleName());
        for(StackTraceElement frame:failure.getStackTrace())if(frame.getClassName().startsWith("ForgeWorker") || frame.getClassName().startsWith("PinnedCombatAssignment") || frame.getClassName().startsWith("forge."))System.err.println("  at "+frame);
        blocked=true;
        String code=failureReference(failure);
        synchronized(gate) {
            pending=null;
            published=JSON.toJson(map("status","blocked","revision",++revision,"failureCode",code,"error","Unsupported or failed Forge input; cancel and start a fresh game"));
        }
    }
    final class Desktop extends GuiDesktop {
        @Override public String getAssetsDir() {return System.getProperty("manatomb.assets")+"/";}
        @Override public void invokeInEdtLater(Runnable task) {
            SwingUtilities.invokeLater(()->{
                try { long before=paintSerial; task.run(); if(paintSerial!=before)publishInput(); } catch(Throwable failure) { fail(failure); }
            });
        }
        @Override public void showBugReportDialog(String title,String text,boolean exit) {
            throw new IllegalStateException("Forge bug report: "+text);
        }
        @Override public int showOptionDialog(String message,String title,forge.localinstance.skin.FSkinProp icon,List<String> options,int defaultOption) {
            throw new UnsupportedOperationException("IGuiBase dialog: "+title);
        }
    }
    void start(Path deck1, Path deck2, int port, Path html) throws Exception {
        this.port=port;
        GuiBase.setInterface(new Desktop());
        FModel.initialize(null,null);
        FModel.getPreferences().setPref(forge.localinstance.properties.ForgePreferences.FPref.PLAYER_NAME,"Human");
        GamePlayerUtil.getGuiPlayer().setName("Human");
        // Production shuffles use Forge randomness; fixtures retain their recorded seed.
        if(focused) MyRandom.setRandom(new Random(20260912));
        Deck h=readDeck(deck1), cpu=readDeck(deck2);
        for(Deck d:List.of(h,cpu)) {
            String problem=DeckFormat.Commander.getDeckConformanceProblem(d);
            if(problem!=null) throw new IllegalArgumentException(d.getName()+": "+problem);
        }
        var hp=RegisteredPlayer.forCommander(h).setPlayer(new LobbyPlayerHuman("Human"));
        var cp=RegisteredPlayer.forCommander(cpu).setPlayer(GamePlayerUtil.createAiPlayer("CPU",0,"Default"));
        GameRules rules=new GameRules(GameType.Commander); rules.setAppliedVariants(EnumSet.of(GameType.Commander));
        Match match=new Match(rules,List.of(hp,cp),"ManaTomb Commander");
        game=match.createGame();
        human=game.getPlayers().get(0); controller=(PlayerControllerHuman)human.getController();
        gui=(IGuiGame)java.lang.reflect.Proxy.newProxyInstance(IGuiGame.class.getClassLoader(),new Class[]{IGuiGame.class},this);
        controller.setGui(gui);
        controller.getInputQueue().addObserver((source,value)->{
            synchronized(gate) { if(pending!=null && pending.reply==null) {pending=null; published=workingView();} }
        });
        game.subscribeToEvents(this);
        HttpServer server=HttpServer.create(new InetSocketAddress("127.0.0.1",port),0);
        server.setExecutor(Executors.newFixedThreadPool(4));
        server.createContext("/", exchange->{
            try {
                if(!Objects.equals(exchange.getRequestHeaders().getFirst("Authorization"),"Bearer "+csrf)) {respond(exchange,403,"text/plain","Invalid host");return;}
                String path=exchange.getRequestURI().getPath();
                if(exchange.getRequestMethod().equals("GET") && path.equals("/")) respond(exchange,200,"text/html","Worker is private");
                else if(exchange.getRequestMethod().equals("GET") && path.equals("/state")) respond(exchange,200,"application/json",published);
                else if(exchange.getRequestMethod().equals("POST") && path.equals("/action")) {
                    
                    byte[] bytes=exchange.getRequestBody().readNBytes(16385);
                    if(bytes.length>16384) {respond(exchange,413,"text/plain","Request too large");return;}
                    JsonObject body=JsonParser.parseString(new String(bytes,StandardCharsets.UTF_8)).getAsJsonObject();
                    String result=accept(body);respond(exchange,200,"application/json",result);
                } else respond(exchange,404,"text/plain","Not found");
            } catch(IllegalArgumentException|IllegalStateException failure) {respond(exchange,409,"application/json",JSON.toJson(map("error",failure.getMessage())));}
            catch(Throwable failure) {failure.printStackTrace();respond(exchange,500,"application/json","{\"error\":\"Adapter failure\"}");}
        });
        server.start();
        System.out.println("BROWSER http://127.0.0.1:"+port+"/ humanController="+controller.getClass().getName());
        new Thread(()->{
            try {match.startGame(game,focused?this::setupFixture:null); synchronized(gate) {gameFinished=true;pending=null; published=JSON.toJson(snapshot("finished",null));} record("natural-result",map("turn",game.getPhaseHandler().getTurn()));}
            catch(Throwable failure) {fail(failure);}
        },"Game V2-011").start();
    }
    static void respond(HttpExchange exchange,int status,String type,String body) throws IOException {
        byte[] bytes=body.getBytes(StandardCharsets.UTF_8);exchange.getResponseHeaders().set("Content-Type",type+"; charset=utf-8");
        exchange.getResponseHeaders().set("Cache-Control","no-store");exchange.getResponseHeaders().set("X-Content-Type-Options","nosniff");
        exchange.sendResponseHeaders(status,bytes.length);try(var out=exchange.getResponseBody()){out.write(bytes);}
    }
    // Production never retains raw game traces, prompt history or private logs.
    void record(String event,Map<String,Object> details) {}
    Deck readDeck(Path path) throws IOException {
        JsonObject input=JsonParser.parseString(Files.readString(path)).getAsJsonObject();
        Deck deck=new Deck(input.get("name").getAsString());
        for(String section:List.of("commanders","main")) {
            CardPool pool=deck.getOrCreate(section.equals("commanders")?DeckSection.Commander:DeckSection.Main);
            for(JsonElement entry:input.getAsJsonArray(section)) {
                JsonObject row=entry.getAsJsonObject();String name=row.get("name").getAsString();
                var card=FModel.getMagicDb().getCommonCards().getCard(name);
                // Resolve the catalog's exact double-faced spelling, never drop unknown cards.
                if(card==null && name.contains(" // "))card=FModel.getMagicDb().getCommonCards().getCard(name.split(" // ")[0]);
                if(card==null)throw new IllegalArgumentException("Card is unavailable in pinned Forge: "+name);
                pool.add(card,row.get("quantity").getAsInt());
            }
        }
        return deck;
    }
    boolean ordinarilyVisible(CardView c) {return c!=null && c.canBeShownTo(human.getView()) && c.canFaceDownBeShownTo(human.getView());}
    boolean visible(CardView c) {return c!=null && (reveals.get().contains(c.getId()) || ordinarilyVisible(c));}
    static String withoutStackMetadata(String text) {
        // WrappedAbility appends localized trigger-object annotations, not only
        // "Card". They contain engine IDs and may describe hidden objects.
        return Objects.toString(text, "").replaceFirst("(?s)\\s+\\[[^\\[\\]\\r\\n]+:.*$", "");
    }
    String cardName(CardView c) {
        // Forge-generated command-zone effects embed the source's engine ID.
        // Keep the permitted source name, but never export that tracking ID.
        return safeText(c.getCurrentState().getName().replaceAll(" \\(\\d+\\)(?='s Effect$)", ""));
    }
    String safeText(String text) {
        if(text==null)return "";
        for(Player player:game.getPlayers())for(ZoneType zone:ZoneType.values())for(Card c:player.getCardsIn(zone)) {
            String engineReference=c.getName()+" ("+c.getId()+")";
            if(text.contains(engineReference)) {
                Integer token=publicIds.get(c.getId());
                String replacement=visible(c.getView())?c.getName()+(token==null?"":" [Card "+token+"]"):"[hidden card]";
                text=text.replace(engineReference,replacement);
            }
        }
        // Raw logs/objects never cross the boundary. Additionally scrub hidden identities from prompt prose.
        Set<String> hidden=new TreeSet<>(Comparator.comparingInt(String::length).reversed().thenComparing(String::compareTo));
        for(Player p:game.getPlayers()) for(ZoneType z:ZoneType.values()) for(Card c:p.getCardsIn(z)) if(!visible(c.getView())) {
            hidden.add(c.getName());
            if(c.getView().getAlternateState()!=null) hidden.add(c.getView().getAlternateState().getName());
        }
        // A hidden copy must not censor an already authorized identity (e.g. basic lands).
        for(Player p:game.getPlayers())for(ZoneType z:ZoneType.values())for(Card c:p.getCardsIn(z))if(visible(c.getView()))hidden.remove(c.getName());
        // Literal clauses from visible Oracle text are already public information.
        // Protect whole clauses only: a hidden name mentioned elsewhere in a prompt
        // must still be redacted (e.g. Plains is a public land type on Grasslands).
        Map<String,String> publicClauses=new LinkedHashMap<>();
        String prefix="\uE000"+UUID.randomUUID()+":";
        for(Player player:game.getPlayers())for(ZoneType zone:ZoneType.values())for(Card c:player.getCardsIn(zone))if(visible(c.getView())) {
            String oracle=Objects.toString(c.getView().getCurrentState().getOracleText(),"");
            for(String clause:oracle.split("(?<=[.!?])\\s+|\\R|: ")) {
                clause=clause.trim();
                if(clause.length()<20 || !text.contains(clause))continue;
                String token=prefix+publicClauses.size()+"\uE001";
                text=text.replace(clause,token);publicClauses.put(token,clause);
            }
        }
        for(String name:hidden) if(name!=null&&!name.isBlank()) text=text.replace(name,"[hidden card]");
        for(var clause:publicClauses.entrySet())text=text.replace(clause.getKey(),clause.getValue());
        return text;
    }
    Map<String,Object> card(CardView cv, Pending p) {
        boolean show=visible(cv);
        Map<String,Object> dto=map("name",show?cardName(cv):"Face-down card","tapped",cv.isTapped(),"attacking",cv.isAttacking(),"blocking",cv.isBlocking());
        if(show) {
            dto.put("temporaryReveal",reveals.get().contains(cv.getId()) && !ordinarilyVisible(cv));
            dto.put("id",publicIds.computeIfAbsent(cv.getId(),k->nextPublicId++));dto.put("mana",cv.getCurrentState().getManaCost().toString());
            dto.put("type",cv.getCurrentState().getType().toString());dto.put("rules",cv.getCurrentState().getOracleText());
            dto.put("token",cv.isToken());dto.put("copy",cv.isCloned());dto.put("backFace",cv.getCurrentState().getState()==forge.card.CardStateName.Backside);
            if(cv.getCurrentState().getType().isCreature())dto.put("pt",cv.getCurrentState().getPower()+"/"+cv.getCurrentState().getToughness());
            dto.put("abilities",safeText(withoutStackMetadata(cv.getCurrentState().getAbilityText())));
            dto.put("selected",highlighted.contains(cv.getId()));
            if(p!=null) p.cards.put((Integer)dto.get("id"),cv);
        }
        if(show || cv.isFaceDown() && cv.getZone()==ZoneType.Battlefield) {
            List<Object> counters=new ArrayList<>();if(cv.getCounters()!=null)for(var entry:cv.getCounters().entrySet())counters.add(map("name",entry.getElement().getName(),"count",entry.getCount()));
            dto.put("counters",counters);dto.put("damage",cv.getDamage());dto.put("summoningSick",cv.isSick());
            CardView attached=cv.getAttachedTo();if(attached!=null)dto.put("attachedTo",visible(attached)?cardName(attached):"Face-down card");
        }
        if(!show && cv.isFaceDown() && cv.getZone()==ZoneType.Battlefield) {
            int id=publicIds.computeIfAbsent(cv.getId(),k->nextPublicId++);dto.put("id",id);if(p!=null)p.cards.put(id,cv);
            dto.put("type",cv.getCurrentState().getType().toString());
            if(cv.getCurrentState().getType().isCreature())dto.put("pt",cv.getCurrentState().getPower()+"/"+cv.getCurrentState().getToughness());
        }
        if(dto.containsKey("id")){dto.put("reference","Card "+dto.get("id"));if(selectable.contains(cv.getId()))dto.put("selectable",true);dto.put("selected",highlighted.contains(cv.getId()));}
        return dto;
    }
    Map<String,Object> phaseStopView() {
        synchronized(gate) {
            Map<String,Object> result=new LinkedHashMap<>();
            for(String seat:List.of("Human","CPU"))result.put(seat,Arrays.stream(PhaseType.values()).filter(phase->!skippedPhases.get(seat).contains(phase)).map(Enum::name).toList());
            return result;
        }
    }
    List<Object> manaPool(Player player) {
        List<Object> result=new ArrayList<>();
        for(byte color:forge.card.mana.ManaAtom.MANATYPES)result.add(map("id",(int)color,"label",forge.card.MagicColor.toLongString(color),"amount",player.getView().getMana(color)));
        return result;
    }
    List<Object> combatView() {
        List<Object> result=new ArrayList<>();var combat=game.getView().getCombat();
        if(combat==null)return result;
        for(CardView attacker:combat.getAttackers()) {
            Integer attackerId=publicIds.get(attacker.getId());if(attackerId==null)continue;
            List<Integer> blockers=new ArrayList<>(),planned=new ArrayList<>();
            if(combat.getBlockers(attacker)!=null)for(CardView c:combat.getBlockers(attacker)){Integer id=publicIds.get(c.getId());if(id!=null)blockers.add(id);}
            if(combat.getPlannedBlockers(attacker)!=null)for(CardView c:combat.getPlannedBlockers(attacker)){Integer id=publicIds.get(c.getId());if(id!=null)planned.add(id);}
            Map<String,Object> row=map("attacker",attackerId,"blockers",blockers,"plannedBlockers",planned);
            GameEntityView defender=combat.getDefender(attacker);
            if(defender instanceof PlayerView player)row.put("defender",map("type","player","id",player.getId()));
            else if(defender instanceof CardView c && publicIds.containsKey(c.getId()))row.put("defender",map("type","card","id",publicIds.get(c.getId())));
            result.add(row);
        }
        return result;
    }
    String workingView() {
        return JSON.toJson(map("status","working","session",incarnation,"revision",++revision,"canRequestConcede",concessionAvailable&&!gameFinished,"concedePending",concessionRequested&&!gameFinished,"phaseStops",phaseStopView()));
    }
    Map<String,Object> snapshot(String status,Pending p) {
        publicIds.clear(); // Entity tokens are scoped to this projection/prompt, never track hidden movements.
        List<Object> players=new ArrayList<>();
        for(Player player:game.getPlayers()) {
            List<Object> zones=new ArrayList<>();
            for(ZoneType zone:List.of(ZoneType.Hand,ZoneType.Battlefield,ZoneType.Graveyard,ZoneType.Exile,ZoneType.Command,ZoneType.Library)) {
                var cards=player.getCardsIn(zone); List<Object> shown=new ArrayList<>();
                List<Card> display=new ArrayList<>();for(Card c:cards)if(visible(c.getView()) || (zone==ZoneType.Battlefield && c.isFaceDown()))display.add(c);
                if(zone==ZoneType.Library || zone==ZoneType.Hand && player!=human)display.sort(Comparator.comparing((Card c)->c.getName()).thenComparingInt(Card::getId));
                for(Card c:display) if(visible(c.getView()) || (zone==ZoneType.Battlefield && c.isFaceDown())) shown.add(card(c.getView(),p));
                // Never export hidden IDs or ordering; visible subsets sorted by public name/id.
                if(zone==ZoneType.Library || zone==ZoneType.Hand && player!=human) shown.sort(Comparator.comparing(Object::toString));
                zones.add(map("zone",zone.name(),"count",cards.size(),"cards",shown));
            }
            if(p!=null)p.players.put(player.getView().getId(),player.getView());
            List<Object> commanders=new ArrayList<>();for(Card c:player.getCommanders())if(visible(c.getView()))commanders.add(map("name",c.getName(),"casts",player.getView().getCommanderCast(c.getView()),"damageToHuman",human.getView().getCommanderDamage(c.getView())));
            players.add(map("id",player.getView().getId(),"name",player==human?"Human":"CPU","life",player.getLife(),"zones",zones,"commanders",commanders,"mana",manaPool(player)));
        }
        List<Object> stack=new ArrayList<>();
        if(game.getView().getStack()!=null) for(StackItemView item:game.getView().getStack()) {
            CardView source=item.getSourceCard();stack.add(map("source",visible(source)?cardName(source):"Hidden source","text",visible(source)?safeText(withoutStackMetadata(item.getText())):""));
        }
        Map<String,Object> dto=map("status",status,"session",incarnation,"revision",++revision,"players",players,"stack",stack,"turn",game.getPhaseHandler().getTurn(),"phase",String.valueOf(game.getPhaseHandler().getPhase()),"turnPlayer",game.getPhaseHandler().getPlayerTurn()==human?"Human":"CPU");
        dto.put("canRequestConcede",concessionAvailable&&!status.equals("finished"));dto.put("concedePending",concessionRequested&&!status.equals("finished"));
        dto.put("phaseStops",phaseStopView());dto.put("combat",combatView());
        dto.put("priorityPlayer",game.getPhaseHandler().getPriorityPlayer()==null?"None":game.getPhaseHandler().getPriorityPlayer()==human?"Human":"CPU");
        if(status.equals("finished")) {dto.put("result",game.getOutcome().isDraw()?"Draw":game.getOutcome().getWinningLobbyPlayer().getName());dto.put("conceded",human.conceded());}
        return dto;
    }
    void publishInput() {
        if(blocked || controller==null || game.isGameOver())return;
        Input input=controller.getInputQueue().getInput();
        if(input==null || paintedInput!=input || controller.getInputProxy().getInput()!=input || input instanceof InputPayMana pay && pay.isActivatingManaAbility()) return;
        synchronized(gate) {
            if(actionInFlight || pending!=null && pending.reply!=null)return;
            if(input instanceof InputPassPriority) {
                concessionAvailable=true;
                if(concessionRequested) {pending=null;controller.concede();return;}
            }
            // Only called after a complete UI dispatch. No HTTP thread reads mutable Forge state.
            Pending p=new Pending();p.kind=input.getClass().getSimpleName();p.input=input;
            p.ok=okEnabled;p.cancel=cancelEnabled && !cancelLabel.startsWith("Undo") && !cancelLabel.equals("Auto");
            p.concede=input instanceof InputPassPriority;
            Map<String,Object> dto=snapshot("input",p);p.revision=revision;dto.put("canConcede",p.concede);
            dto.put("prompt",map("id",p.id,"kind",p.kind,"text",safeText(promptText),"ok",okLabel,"cancel",cancelLabel,"okEnabled",p.ok,"cancelEnabled",p.cancel));
            List<Object> mana=new ArrayList<>();if(input instanceof InputPayMana)for(byte color:forge.card.mana.ManaAtom.MANATYPES){int count=human.getView().getMana(color);if(count>0){p.mana.add((int)color);mana.add(map("id",(int)color,"label",forge.card.MagicColor.toLongString(color),"amount",count));}}dto.put("mana",mana);
            dto.put("actionError",actionError);pending=p;shownInput=input;published=JSON.toJson(dto);
            record("projection",dto);
            record("input",map("revision",revision,"kind",p.kind,"turn",game.getPhaseHandler().getTurn()));
        }
    }
    String label(Object option) {
        if(option instanceof CardView c)return visible(c)?cardName(c)+(publicIds.containsKey(c.getId())?" [Card "+publicIds.get(c.getId())+"]":""):"Hidden card";
        if(option instanceof SpellAbilityView a)return abilityLabel(a);
        if(option instanceof ReplacementEffectView a)return sourcedLabel(a.getHostCard(),a.getDescription());
        if(option instanceof StaticAbility a)return label(a.getView()); // Local Forge passes the original object; project its view only.
        if(option instanceof StaticAbilityView a)return sourcedLabel(a.getHostCard(),a.getDescription());
        if(option instanceof CardFaceView face)return face.getTranslatedName(); // Explicit catalog-name choice, not a zone reveal.
        if(option instanceof CardView.CardStateView state)return visible(state.getCard())
            ? label(state.getCard())+" — "+safeText(state.getName())+": "+safeText(state.getOracleText()) : "Hidden card face";
        if(option instanceof CounterType counter)return safeText(counter.getName());
        if(option instanceof forge.card.MagicColor.Color color)return color.getTranslatedName();
        if(option instanceof Pair<?,?> pair && pair.getLeft() instanceof SpellAbilityStackInstance stack) {
            Object target=pair.getRight();
            String targetLabel;
            if(target instanceof Card card)targetLabel=label(card.getView());
            else if(target instanceof Player player)targetLabel=label(player.getView());
            else if(target instanceof SpellAbility ability)targetLabel=abilityLabel(ability.getView());
            else throw new UnsupportedOperationException("Unmapped redirected target type");
            return targetLabel+" — "+sourcedLabel(stack.getSpellAbility().getHostCard().getView(),stack.getStackDescription());
        }
        if(option instanceof PlayerView p)return p.equals(human.getView())?"Human":"CPU";
        if(option instanceof forge.item.PaperCard c)return c.getName();
        if(option instanceof OptionalCostValue c)return safeText(c.toString());
        if(option instanceof Byte color)return forge.card.MagicColor.toLongString(color);
        if(option instanceof String || option instanceof Number || option instanceof Enum<?>)return revealText.get()?option.toString():safeText(option.toString());
        throw new UnsupportedOperationException("Unmapped choice type "+option.getClass().getName());
    }
    String sourcedLabel(CardView source,String description) {
        if(!visible(source))return "Hidden ability";
        String text=safeText(withoutStackMetadata(description)).trim();
        if(text.isBlank())text=safeText(source.getCurrentState().getOracleText());
        if(text.isBlank())throw new UnsupportedOperationException("Ability lacks authorized explanatory text");
        return label(source)+" — "+text;
    }
    String publicDamageEntity(String engineText) {
        // Only translate an exact entity already present in this authorized
        // projection. Never expose raw trigger metadata or tracking IDs.
        for(Player player:game.getPlayers()) {
            if(engineText.equals(player.toString()))return player==human?"Human":"CPU";
            for(ZoneType zone:ZoneType.values())for(Card card:player.getCardsIn(zone))
                if(publicIds.containsKey(card.getId()) && visible(card.getView()) && engineText.equals(card.toString()))return label(card.getView());
        }
        return null;
    }
    String publicDamageContext(String description) {
        // Pinned TriggerDamageDone.getImportantStackObjects supplies these three
        // localized fields. Accept only this bounded shape, then authorize each
        // entity separately; all other stack annotations stay stripped.
        var localizer=forge.util.Localizer.getInstance();
        String prefix=java.util.regex.Pattern.quote(" ["+localizer.getMessage("lblDamageSource")+": ");
        String middle=java.util.regex.Pattern.quote(", "+localizer.getMessage("lblDamaged")+": ");
        String amount=java.util.regex.Pattern.quote(", "+localizer.getMessage("lblAmount")+": ");
        var match=java.util.regex.Pattern.compile(prefix+"(.+)"+middle+"(.+)"+amount+"([0-9]{1,10})\\]$").matcher(Objects.toString(description,""));
        if(!match.find())return "";
        String source=publicDamageEntity(match.group(1)),target=publicDamageEntity(match.group(2));
        if(source==null || target==null)return "";
        return "Damage from "+source+" to "+target+": "+match.group(3);
    }
    String abilityLabel(SpellAbilityView ability) {
        CardView source=ability.getHostCard();
        if(!visible(source))return "Hidden ability";
        String raw=Objects.toString(ability.getDescription(), "");
        String description=safeText(withoutStackMetadata(raw)).trim();
        // A secondary trigger can have no prose upstream, only a stack annotation.
        // Use the authorized current face's card text as context, never infer a rule.
        boolean missing=description.replaceAll("\\[[^\\]]*\\]", "").isBlank();
        String result=label(source)+" — "+description;
        String damageContext=publicDamageContext(raw);if(!damageContext.isEmpty())result+=" · "+damageContext;
        if(missing) {
            String rules=safeText(source.getCurrentState().getOracleText());
            if(rules.isBlank())throw new UnsupportedOperationException("Ability lacks authorized explanatory text");
            result+=" · Source rules: "+rules;
            record("ability-context",map("source",source.getCurrentState().getName(),
                "viewDescription",safeText(raw),"translatedDescription",description,
                "scrubChangedDescription",!raw.equals(safeText(raw)),
                "metadataRemoved",!raw.equals(withoutStackMetadata(raw)),
                "context","Current visible face's Forge Oracle text"));
        }
        return result;
    }
    List<Integer> ask(String kind,String message,List<?> options,int min,int max) throws Exception {
        return askValues(kind,message,options,min,max,"selection",null,Map.of());
    }
    List<Integer> askValues(String kind,String message,List<?> options,int min,int max,String valueType,java.util.function.Consumer<List<Integer>> validator,Map<String,Object> extra) throws Exception {
        Pending p=new Pending();p.kind=kind;p.options=List.copyOf(options);p.min=min;p.max=max;p.reply=new CompletableFuture<>();p.valueType=valueType;p.validator=validator;
        synchronized(gate) {
            Map<Integer,Integer> priorReferences=new HashMap<>(publicIds);
            Map<String,Object> dto=snapshot("choice",p);p.revision=revision;
            message=refreshReferences(message,priorReferences);
            List<Object> opts=new ArrayList<>();Set<String> abilityLabels=new HashSet<>();
            for(int i=0;i<options.size();i++) {
                String text=label(options.get(i));
                if(options.get(i) instanceof String)text=refreshReferences(text,priorReferences);
                if(options.get(i) instanceof SpellAbilityView && !abilityLabels.add(text))
                    throw new UnsupportedOperationException("Indistinguishable ability options need additional authorized context");
                opts.add(map("id",i,"label",text));
            }
            Map<String,Object> prompt=map("id",p.id,"kind",kind,"text",safeText(message),"min",min,"max",max,"options",opts,"ordering",kind.equals("orderSimultaneousAbilities") || kind.equals("orderCards"),"valueType",valueType);
            prompt.putAll(extra);dto.put("prompt",prompt);
            pending=p;published=JSON.toJson(dto);record("projection",dto);record("choice",map("kind",kind,"revision",revision,"thread",Thread.currentThread().getName()));
        }
        List<Integer> result=p.reply.get();
        record("choice-unblocked",map("kind",kind,"thread",Thread.currentThread().getName()));
        return result;
    }
    String refreshReferences(String text,Map<Integer,Integer> previous) {
        Map<Integer,Integer> translated=new HashMap<>();
        for(var entry:previous.entrySet())if(publicIds.containsKey(entry.getKey()))translated.put(entry.getValue(),publicIds.get(entry.getKey()));
        var matcher=java.util.regex.Pattern.compile("\\[Card (\\d+)\\]").matcher(text);
        return matcher.replaceAll(match->{Integer current=translated.get(Integer.valueOf(match.group(1)));return current==null?"[card]":"[Card "+current+"]";});
    }
    static int strictInt(JsonElement e) {
        if(e==null || !e.isJsonPrimitive() || !e.getAsJsonPrimitive().isNumber() || !e.getAsString().matches("-?(0|[1-9][0-9]*)"))throw new IllegalArgumentException("Expected an integer");
        try{return Integer.parseInt(e.getAsString());}catch(NumberFormatException failure){throw new IllegalArgumentException("Integer out of range");}
    }
    Object assignCombat(Object[] a)throws Exception {
        CardView attacker=(CardView)a[0];List<CardView> targets=PinnedCombatAssignment.targets(attacker,(List<CardView>)a[1],(GameEntityView)a[3],(Boolean)a[4]);
        if((Boolean)a[5] && one("skipCombatAssignment","Assign combat damage from "+label(attacker)+" or use Forge's offered skip?",List.of("Assign damage","Skip this assignment"),false).equals("Skip this assignment"))return null;
        List<Object> labels=new ArrayList<>();for(CardView c:targets)labels.add(c==null?label(a[3]):label(c));
        int total=(Integer)a[2];
        List<Integer> amounts=askValues("assignCombatDamage","Assign combat damage from "+label(attacker)+". Forge's assignment constraints apply.",labels,0,0,"amounts",values->PinnedCombatAssignment.validate(attacker,targets,(GameEntityView)a[3],(Boolean)a[4],total,game.getView().getPoisonCountersToLose(),values),map("total",total,"minimumEach",0,"limits",Collections.nCopies(targets.size(),total)));
        Map<CardView,Integer> result=new HashMap<>();for(int i=0;i<targets.size();i++)result.put(targets.get(i),amounts.get(i));return result;
    }
    Object assignAmounts(Object[] a)throws Exception {
        Map<Object,Integer> offered=(Map<Object,Integer>)a[1];List<Object> targets=new ArrayList<>(offered.keySet());int total=(Integer)a[2];int minimum=(Boolean)a[3]?1:0;
        List<Integer> limits=new ArrayList<>();for(Object target:targets)limits.add(offered.get(target)==null?total:offered.get(target));
        List<Integer> amounts=askValues("assignGenericAmount",(String)a[4],targets,0,0,"amounts",values->{
            if(values.size()!=targets.size() || values.stream().mapToLong(Integer::longValue).sum()!=total)throw new IllegalArgumentException("Assign exactly the offered total");
            for(int i=0;i<values.size();i++)if(values.get(i)<minimum || values.get(i)>limits.get(i))throw new IllegalArgumentException("Outside Forge target range");
        },map("total",total,"minimumEach",minimum,"limits",limits));
        Map<Object,Integer> result=new HashMap<>();for(int i=0;i<targets.size();i++)result.put(targets.get(i),amounts.get(i));return result;
    }
    Object orderSimultaneous(Object[] a) throws Exception {
        // Pin-specific boundary: no generic card/sideboard/replacement ordering support.
        boolean caller=StackWalker.getInstance().walk(frames->frames.anyMatch(f->
            f.getClassName().equals(PlayerControllerHuman.class.getName()) && f.getMethodName().equals("orderSimultaneousSa")));
        if(!caller) {
            if(a.length!=9 || (Boolean)a[7])throw new UnsupportedOperationException("Sideboarding outside Commander session");
            List<Object> offered=new ArrayList<>();if(a[4]!=null)offered.addAll((List<?>)a[4]);if(a[5]!=null)offered.addAll((List<?>)a[5]);
            int remainingMin=(Integer)a[2],remainingMax=(Integer)a[3];
            if(remainingMin<0 || remainingMax<remainingMin)throw new UnsupportedOperationException("Unsupported order bounds");
            int min=Math.max(0,offered.size()-remainingMax),max=offered.size()-remainingMin;
            List<Object> result=new ArrayList<>();
            for(int i:ask(min==offered.size()?"orderCards":"selectOrderedCards",(String)a[0]+" — "+(String)a[1]+". Choose the requested list in order. Position 1 is the first returned item; follow the Forge destination instructions above.",offered,min,max))result.add(offered.get(i));
            if(min!=offered.size() && result.size()>1) {
                List<Object> ordered=new ArrayList<>();
                for(int i:ask("orderCards",(String)a[0]+" — "+(String)a[1]+". Arrange the selected list. Position 1 is the first returned item.",result,result.size(),result.size()))ordered.add(result.get(i));
                result=ordered;
            }
            return new IGuiGame.OrderResult<>(result,false);
        }
        if(a.length!=9 || !Integer.valueOf(0).equals(a[2]) || !Integer.valueOf(0).equals(a[3])
            || a[6]!=null || !Boolean.FALSE.equals(a[7]) || !Boolean.TRUE.equals(a[8]))
            throw new UnsupportedOperationException("Only simultaneous-ability ordering is mapped");
        List<?> source=a[4]==null?List.of():(List<?>)a[4], dest=a[5]==null?List.of():(List<?>)a[5];
        if(!source.isEmpty() && !dest.isEmpty())throw new UnsupportedOperationException("Mixed ordering lists are not mapped");
        List<?> offered=source.isEmpty()?dest:source;
        if(offered.size()<2 || new HashSet<>(offered).size()!=offered.size()
            || offered.stream().anyMatch(o->!(o instanceof SpellAbilityView v) || !visible(v.getHostCard())))
            throw new UnsupportedOperationException("Ordering needs distinct, authorized ability views");
        List<Integer> selected=ask("orderSimultaneousAbilities",
            "Choose every ability exactly once. Position 1 resolves next, then position 2. Forge places them on the stack in reverse order; later responses can resolve before them. This choice is not remembered.",
            offered,offered.size(),offered.size());
        List<SpellAbilityView> ordered=new ArrayList<>();
        for(int i:selected)ordered.add((SpellAbilityView)offered.get(i));
        record("order-returned",map("selected",selected,"rememberDecision",false));
        return new IGuiGame.OrderResult<SpellAbilityView>(ordered,false);
    }
    String permittedOrder(List<CardView> cards) {
        // Compress indistinguishable hidden cards; never export their identities or tracking order.
        List<String> shown=new ArrayList<>();int hidden=0;
        for(CardView card:cards) {
            if(!visible(card)){hidden++;continue;}
            if(hidden>0){shown.add(hidden+" hidden cards");hidden=0;}
            shown.add(label(card));
        }
        if(hidden>0)shown.add(hidden+" hidden cards");
        return String.join(" → ",shown);
    }
    Object one(String kind,String text,List<?> options,boolean optional) throws Exception {
        // IGuiGame explicitly defines null for an absent/empty one/oneOrNone list.
        if(options==null || options.isEmpty())return null;
        var selected=ask(kind,text,options,optional?0:1,1);return selected.isEmpty()?null:options.get(selected.get(0));
    }
    String accept(JsonObject body) {
        
        String request=body.get("request").getAsString(),action=body.get("action").getAsString();
        Pending p;
        synchronized(gate) {
            if(consumed.containsKey(request)) {
                if(!consumed.get(request).equals(body.toString()))throw new IllegalArgumentException("Repeated request with different payload");
                return "{\"accepted\":true}";
            }
            // Session termination intent is independent of a particular prompt.
            // Only set a flag here: HTTP never reads/mutates Forge game objects.
            if(action.equals("requestConcede")) {
                if(!incarnation.equals(body.get("session").getAsString()) || !concessionAvailable || gameFinished || blocked)throw new IllegalArgumentException("Concession request is unavailable for this session");
                consumed.put(request,body.toString());while(consumed.size()>128)consumed.remove(consumed.keySet().iterator().next());
                concessionRequested=true;
                JsonObject view=JsonParser.parseString(published).getAsJsonObject();view.addProperty("concedePending",true);published=view.toString();
                SwingUtilities.invokeLater(()->{try{publishInput();}catch(Throwable failure){fail(failure);}});
                return "{\"accepted\":true}";
            }
            if(action.equals("setPhaseStop")) {
                if(body.has("revision") && strictInt(body.get("revision"))!=0 || body.has("prompt") && !body.get("prompt").getAsString().isEmpty())throw new IllegalArgumentException("Phase stop is a session preference");
                if(!incarnation.equals(body.get("session").getAsString()) || gameFinished || blocked)throw new IllegalArgumentException("Phase stops are unavailable for this session");
                String seat=body.get("seat").getAsString();PhaseType phase;
                try{phase=PhaseType.valueOf(body.get("phase").getAsString());}catch(RuntimeException failure){throw new IllegalArgumentException("Unknown phase");}
                if(!skippedPhases.containsKey(seat) || !body.has("enabled") || !body.get("enabled").isJsonPrimitive() || !body.getAsJsonPrimitive("enabled").isBoolean())throw new IllegalArgumentException("Invalid phase stop");
                boolean enabled=body.get("enabled").getAsBoolean();
                if(enabled)skippedPhases.get(seat).remove(phase);else skippedPhases.get(seat).add(phase);
                consumed.put(request,body.toString());while(consumed.size()>128)consumed.remove(consumed.keySet().iterator().next());
                JsonObject view=JsonParser.parseString(published).getAsJsonObject();view.add("phaseStops",JSON.toJsonTree(phaseStopView()));published=view.toString();
                // Use Forge's own yield update to reconsider an already displayed
                // priority input. A forced choice remains untouched.
                SwingUtilities.invokeLater(()->{try{
                    Player player=seat.equals("Human")?human:game.getPlayers().stream().filter(x->x!=human).findFirst().orElseThrow();
                    controller.applyYieldUpdate(new YieldUpdate.SkipPhase(player.getView(),phase,!enabled));
                }catch(Throwable failure){fail(failure);}});
                return "{\"accepted\":true}";
            }
            p=pending;
            if(p==null || !incarnation.equals(body.get("session").getAsString()) || p.revision!=body.get("revision").getAsLong() || !p.id.equals(body.get("prompt").getAsString()))throw new IllegalArgumentException("Stale prompt or revision");
            if(p.reply!=null) {
                if(!action.equals("reply"))throw new IllegalArgumentException("Expected reply");
                List<Integer> selected=new ArrayList<>();
                if(!p.valueType.equals("selection")) {
                    if(p.valueType.equals("integer"))selected.add(strictInt(body.get("value")));
                    else if(p.valueType.equals("amounts")){for(JsonElement entry:body.getAsJsonArray("amounts"))selected.add(strictInt(entry));}
                    else if(p.valueType.equals("text")){String value=body.get("text").getAsString();if(p.kind.equals("numericText")&&!value.matches("[0-9]+"))throw new IllegalArgumentException("Forge requested numeric text");if(value.length()>200 || value.chars().anyMatch(c->Character.isISOControl(c)))throw new IllegalArgumentException("Invalid text");value.chars().forEach(selected::add);}
                    else throw new IllegalArgumentException("Unknown value input");
                    if(p.validator!=null)p.validator.accept(selected);
                } else if(p.kind.equals("orderSimultaneousAbilities") || p.kind.equals("orderCards")) {
                    if(!body.has("selected") || !body.get("selected").isJsonArray() || body.has("rememberDecision"))throw new IllegalArgumentException("Expected an explicit order without remembering");
                    for(JsonElement e:body.getAsJsonArray("selected")) {
                        if(!e.isJsonPrimitive() || !e.getAsJsonPrimitive().isNumber() || !e.getAsString().matches("0|[1-9][0-9]*"))throw new IllegalArgumentException("Invalid order entry");
                        try {selected.add(Integer.parseInt(e.getAsString()));}catch(NumberFormatException failure){throw new IllegalArgumentException("Invalid order entry");}
                    }
                } else for(JsonElement e:body.getAsJsonArray("selected")) {if(!e.isJsonPrimitive() || !e.getAsJsonPrimitive().isNumber() || !e.getAsString().matches("0|[1-9][0-9]*"))throw new IllegalArgumentException("Invalid choice entry");selected.add(Integer.parseInt(e.getAsString()));}
                if(p.valueType.equals("selection") && (selected.size()<p.min||selected.size()>p.max||new HashSet<>(selected).size()!=selected.size()||selected.stream().anyMatch(i->i<0||i>=p.options.size())))throw new IllegalArgumentException("Invalid choice");
                consumed.put(request,body.toString());while(consumed.size()>128)consumed.remove(consumed.keySet().iterator().next());pending=null;published=workingView();
                record("browser-reply",map("kind",p.kind,"selected",selected));p.reply.complete(selected);
            } else {
                if(!Set.of("ok","cancel","card","player","mana","concede").contains(action))throw new IllegalArgumentException("Invalid action");
                if(action.equals("concede")&&!p.concede || action.equals("ok")&&!p.ok || action.equals("cancel")&&!p.cancel)throw new IllegalArgumentException("Disabled control");
                int id=body.has("id")?strictInt(body.get("id")):-1;
                if(action.equals("mana")&&!p.mana.contains(id))throw new IllegalArgumentException("Unoffered mana");
                if(action.equals("card")&&!p.cards.containsKey(id) || action.equals("player")&&!p.players.containsKey(id))throw new IllegalArgumentException("Unoffered entity");
                
                consumed.put(request,body.toString());while(consumed.size()>128)consumed.remove(consumed.keySet().iterator().next());pending=null;published=workingView();
                actionInFlight=true;
                SwingUtilities.invokeLater(()->{
                    try {
                        if(controller.getInputQueue().getInput()!=p.input || controller.getInputProxy().getInput()!=p.input) throw new IllegalArgumentException("Input advanced before dispatch");
                        invalidAction=false;actionError="";
                        boolean accepted=true;
                        switch(action) {
                            case "ok" -> controller.selectButtonOk();
                            case "cancel" -> controller.selectButtonCancel();
                            case "card" -> {
                                clickedCard.set(p.cards.get(id).getId());
                                try{accepted=controller.selectCard(p.cards.get(id),null,null);}finally{clickedCard.remove();}
                            }
                            case "player" -> controller.selectPlayer(p.players.get(id),null);
                            case "mana" -> controller.useMana((byte)id);
                            // Same controller path as the pinned desktop, only while the
                            // engine is waiting on a stable priority input. CPU work can
                            // always be cancelled by the supervisor, without inventing a result.
                            case "concede" -> controller.concede();
                        }
                        if(!accepted||invalidAction)actionError="Forge rejected that action; choose a legal control.";
                        record("browser-action",map("input",p.kind,"action",action,"accepted",accepted&&!invalidAction));
                        } catch(Throwable failure){fail(failure);}
                    finally {
                        synchronized(gate){actionInFlight=false;}
                        if(controller.getInputQueue().getInput()==p.input)publishInput();
                    }
                });
            }
        }
        return "{\"accepted\":true}";
    }
    @Subscribe public void event(GameEvent event) {
        // Event class alone is safe; full event.toString()/game logs are intentionally excluded.
        if(event instanceof GameEventSpellResolved resolved && visible(resolved.spell().getHostCard()))
            record("resolved-public-source",map("source",resolved.spell().getHostCard().getCurrentState().getName(),"fizzled",resolved.hasFizzled()));
        if(event instanceof GameEventSpellAbilityCast || event instanceof GameEventGameOutcome || event instanceof GameEventZone || event instanceof GameEventCombatChanged) record("engine-event",map("type",event.getClass().getSimpleName()));
    }
    @Override public Object invoke(Object proxy,Method method,Object[] args) throws Throwable {
        String name=method.getName();Object[] a=args==null?new Object[0]:args;
        if(method.isDefault())return InvocationHandler.invokeDefault(proxy,method,a);
        switch(name) {
            case "toString":return "ForgeWorkerGui";
            case "hashCode":return System.identityHashCode(proxy);
            case "equals":return proxy==a[0];
            case "getGameView":return game.getView();
            case "isLibgdxPort", "isNetGame", "isGamePaused":return false;
            case "isUiSetToSkipPhase":synchronized(gate){return skippedPhases.get(a[0].equals(human.getView())?"Human":"CPU").contains((PhaseType)a[1]);}
            case "getDayTime":return "";
            case "getGameSpeed":return forge.gui.control.PlaybackSpeed.NORMAL;
            case "isSelecting":return !selectable.isEmpty();
            case "showPromptMessage":promptText=(String)a[1];paintedInput=controller.getInputQueue().getInput();paintSerial++;return null;
            case "updateButtons":okLabel=(String)a[1];cancelLabel=(String)a[2];okEnabled=(Boolean)a[3];cancelEnabled=(Boolean)a[4];return null;
            case "setSelectables":selectable.clear();for(Object c:(Iterable<?>)a[0])selectable.add(((CardView)c).getId());return null;
            case "clearSelectables":selectable.clear();return null;
            case "setHighlighted":for(Object entity:(Iterable<?>)a[0]){int id=((GameEntityView)entity).getId();if((Boolean)a[1])highlighted.add(id);else highlighted.remove(id);}return null;
            case "flashIncorrectAction":invalidAction=true;return null;
            case "reveal": {
                List<?> items=(List<?>)a[1];
                for(Object item:items)if(item instanceof CardView c)reveals.get().add(c.getId());
                revealText.set(true);
                try {ask(name,"Forge authorized reveal",items,0,0);} finally {reveals.get().clear();revealText.set(false);}
                return null;
            }
            case "order":return orderSimultaneous(a);
            case "getAbilityToPlay": {
                List<SpellAbilityView> offered=(List<SpellAbilityView>)a[1];
                if(offered!=null && offered.size()==1 && !offered.get(0).promptIfOnlyPossibleAbility()) {
                    // Match the pinned desktop's sole-action shortcut after a
                    // deliberate card click. Forge's playable view is authoritative.
                    if(a[0] instanceof CardView host && Objects.equals(clickedCard.get(),host.getId()))return offered.get(0).canPlay()?offered.get(0):null;
                    // Forge checks additional cost variants after the initial
                    // action selection, including already-confirmed triggers.
                    // A singleton here is the unchanged action, not a new choice.
                    boolean continuation=StackWalker.getInstance().walk(frames->frames.anyMatch(frame->frame.getClassName().equals("forge.game.player.PlaySpellAbility") && frame.getMethodName().equals("chooseOptionalAdditionalCosts")));
                    if(continuation)return offered.get(0);
                }
                return one(name,"Choose an ability",offered,true);
            }
            case "showConfirmDialog":return one(name,(String)a[0],List.of(a[2],a[3]),false).equals(a[2]);
            case "confirm":return one(name,(String)a[1],(List<?>)a[3],false).equals(((List<?>)a[3]).get(0));
            case "one":return one(name,(String)a[0],(List<?>)a[1],false);
            case "oneOrNone":return one(name,(String)a[0],(List<?>)a[1],true);
            case "manipulateCardList": {
                List<CardView> all=new ArrayList<>(),movable=new ArrayList<>();for(Object c:(Iterable<?>)a[1])all.add((CardView)c);for(Object c:(Iterable<?>)a[2])movable.add((CardView)c);
                for(;;){List<Object> choices=new ArrayList<>();choices.add("Finish arrangement");choices.addAll(movable);
                    Object selected=one(name,(String)a[0]+". Current order: "+permittedOrder(all),choices,false);
                    if(selected instanceof String)return all;
                    CardView card=(CardView)selected;
                    if((Boolean)a[5]){int max=all.size()-1;int at=askValues(name,"Place "+label(card)+" at position (0 is top)",List.of(),0,0,"integer",values->{if(values.get(0)<0||values.get(0)>max)throw new IllegalArgumentException("Invalid position");},map("lower",0,"upper",max)).get(0);all.remove(card);all.add(at,card);}
                    else {List<String> locations=new ArrayList<>();if((Boolean)a[3])locations.add("Top");if((Boolean)a[4])locations.add("Bottom");String location=(String)one(name,"Place "+label(card),locations,false);all.remove(card);if(location.equals("Top"))all.add(0,card);else all.add(card);}
                }
            }
            case "chooseSingleEntityForEffect": {
                DelayedReveal reveal=(DelayedReveal)a[2];if(reveal!=null)for(CardView c:reveal.getCards())reveals.get().add(c.getId());
                try{return one(name,(String)a[0],(List<?>)a[1],(Boolean)a[3]);}finally{reveals.get().clear();}
            }
            case "getInteger": {
                int min=(Integer)a[1],max=(Integer)a[2];
                return askValues(name,(String)a[0],List.of(),0,0,"integer",values->{if(values.size()!=1 || values.get(0)<min || values.get(0)>max)throw new IllegalArgumentException("Outside Forge's offered range");},map("lower",min,"upper",max)).get(0);
            }
            case "chooseEntitiesForEffect": {
                DelayedReveal reveal=(DelayedReveal)a[4];
                if(reveal!=null)for(CardView c:reveal.getCards())reveals.get().add(c.getId());
                try {List<?> options=(List<?>)a[1];List<Object> result=new ArrayList<>();for(int i:ask(name,(String)a[0],options,(Integer)a[2],Math.min(options.size(),(Integer)a[3])))result.add(options.get(i));return result;}finally{reveals.get().clear();}
            }
            case "assignCombatDamage":return assignCombat(a);
            case "assignGenericAmount":return assignAmounts(a);
            case "many": {
                List<Object> options=new ArrayList<>((List<?>)a[4]);if(a[5]!=null)options.addAll((List<?>)a[5]);
                List<Object> chosen=new ArrayList<>();for(int i:ask(name,(String)a[0]+" — "+a[1],options,(Integer)a[2],Math.min((Integer)a[3],options.size())))chosen.add(options.get(i));
                if(chosen.size()<2)return chosen;
                List<Object> ordered=new ArrayList<>();for(int i:ask("orderCards","Arrange selected cards in Forge's requested order: "+a[0],chosen,chosen.size(),chosen.size()))ordered.add(chosen.get(i));return ordered;
            }
            case "getChoices": {
                List<?> options=(List<?>)a[3];List<Object> out=new ArrayList<>();for(int i:ask(name,(String)a[0],options,(Integer)a[1],(Integer)a[2]))out.add(options.get(i));return out;
            }
            case "showOptionDialog":return ask(name,(String)a[0],(List<?>)a[3],1,1).get(0);
            case "showInputDialog": {
                if(a[4]!=null)return one(name,(String)a[0],(List<?>)a[4],true);
                StringBuilder text=new StringBuilder();for(int c:askValues((Boolean)a[5]?"numericText":name,(String)a[0],List.of(),0,0,"text",null,map("numeric",a[5])))text.append((char)c);
                if((Boolean)a[5]&&!text.toString().matches("[0-9]+"))throw new IllegalArgumentException("Forge requested numeric text");return text.toString();
            }
            case "insertInList": {
                List<Object> result=new ArrayList<>((List<?>)a[2]);int max=result.size();
                int at=askValues(name,(String)a[0]+" — insert "+label(a[1])+" at a position (0 is first)",List.of(),0,0,"integer",values->{if(values.get(0)<0||values.get(0)>max)throw new IllegalArgumentException("Invalid position");},map("lower",0,"upper",max)).get(0);
                result.add(at,a[1]);return result;
            }
            case "message", "showErrorDialog":ask(name,(String)a[0],List.of("Acknowledge"),1,1);return null;
            case "tempShowZones":return a[1];
            case "openZones":return new PlayerZoneUpdates();
            case "getGamestate":throw new UnsupportedOperationException("GameState export disabled");
        }
        if(method.getReturnType()==void.class && Set.of("setGameView","setOriginalGameController","setGameController","setSpectator","openView","afterGameEnd","showCombat","alertUser","updatePhase","updateTurn","updatePlayerControl","enableOverlay","disableOverlay","finishGame","showManaPool","hideManaPool","updateStack","notifyStackAddition","notifyStackRemoval","handleLandPlayed","handleGameEvent","hideZones","updateZones","updateSingleCard","updateCards","updateRevealedCards","refreshCardDetails","refreshField","updateManaPool","updateLives","updateShards","updateDependencies","setPanelSelection","setCard","setPlayerAvatar","restoreOldZones","setWeaklySelectable","clearWeaklySelectable","setGamePause","setGameSpeed","updateDayTime","awaitNextInput","cancelAwaitNextInput","showWaitingTimer","updateAutoPassPrompt","setCurrentPlayer","applyDelta","applyYieldUpdate","setNetGame").contains(name)) return null;
        throw new UnsupportedOperationException("Unmapped GUI method: "+name);
    }
    // One-time fixture arrangement through Forge APIs before the first turn begins.
    // No fixture mutation is reachable from HTTP or after play starts.
    Card place(Player player,String name,ZoneType destination) {
        Card found=null;
        for(Card c:player.getCardsIn(ZoneType.Library))if(c.getName().equals(name)){found=c;break;}
        if(found==null)throw new IllegalStateException("Fixture missing "+name);
        Card placed=game.getAction().moveTo(destination,found,null,new HashMap<>());
        if(destination==ZoneType.Battlefield)placed.setSickness(false);
        return placed;
    }
    void setupOrderingFixture() {
        for(Player player:game.getPlayers())for(Card c:new ArrayList<Card>(player.getCardsIn(ZoneType.Hand)))game.getAction().moveTo(ZoneType.Library,c,null,new HashMap<>());
        Card commander=human.getCardsIn(ZoneType.Command).stream().filter(c->c.getName().equals("Arahbo, Roar of the World")).findFirst().orElseThrow();
        game.getAction().moveTo(ZoneType.Battlefield,commander,null,new HashMap<>());commander.setSickness(false);
        Card cat=place(human,"Leonin Shikari",ZoneType.Battlefield);
        Card equipment=place(human,"Sword of the Animist",ZoneType.Battlefield);equipment.attachToEntity(cat,null);
        for(int i=0;i<3;i++){place(human,"Forest",ZoneType.Battlefield);place(human,"Plains",ZoneType.Battlefield);}
        for(int i=0;i<3;i++)place(game.getPlayers().get(1),"Plains",ZoneType.Hand);
        record("fixture-start",map("setup","One-time real Arahbo, equipped Leonin, six lands; no later fixture mutations","ordering",true));
    }
    void setupChoicesFixture() {
        for(Player player:game.getPlayers())for(Card c:new ArrayList<Card>(player.getCardsIn(ZoneType.Hand)))game.getAction().moveTo(ZoneType.Library,c,null,new HashMap<>());
        for(int i=0;i<10;i++)place(human,"Forest",ZoneType.Battlefield);
        for(String name:List.of("Doubling Season","Hardened Scales","Grizzly Bears"))place(human,name,ZoneType.Battlefield);
        for(String name:List.of("Walking Ballista","Vines of Vastwood","Evolution Charm"))place(human,name,ZoneType.Hand);
        Player cpu=game.getPlayers().get(1);for(int i=0;i<3;i++)place(cpu,"Plains",ZoneType.Hand);
        cpu.setLife(8,null);
        record("fixture-start",map("setup","One-time X, kicker, modes and counter replacement initial state; all subsequent rules belong to Forge"));
    }
    void setupVerificationFixture() {
        // Local QA initial state only: real cards/prompts, no later scripted turns or choices.
        for(Player player:game.getPlayers())for(Card c:new ArrayList<Card>(player.getCardsIn(ZoneType.Hand)))game.getAction().moveTo(ZoneType.Library,c,null,new HashMap<>());
        for(int i=0;i<7;i++){place(human,"Forest",ZoneType.Battlefield);place(human,"Mountain",ZoneType.Battlefield);place(human,"Swamp",ZoneType.Battlefield);}
        for(int i=0;i<16;i++)place(human,"Persistent Petitioners",ZoneType.Battlefield);
        place(human,"Colossal Dreadmaw",ZoneType.Battlefield);
        place(human,"Goblin War Drums",ZoneType.Battlefield);
        place(human,"Rolling Thunder",ZoneType.Hand);place(human,"Cabal Therapy",ZoneType.Hand);
        Player cpu=game.getPlayers().get(1);
        place(cpu,"Siege Mastodon",ZoneType.Battlefield);place(cpu,"Thraben Purebloods",ZoneType.Battlefield);
        place(cpu,"Savannah Lions",ZoneType.Battlefield);place(cpu,"Vampire Noble",ZoneType.Hand);
        for(int i=0;i<6;i++)place(cpu,"Plains",ZoneType.Hand);
        cpu.setLife(5,null);
    }
    void setupFixture() {
        if(verificationFixture){setupVerificationFixture();return;}
        if(choicesFixture){setupChoicesFixture();return;}
        if(orderingFixture){setupOrderingFixture();return;}
        for(Player player:game.getPlayers())for(Card c:new ArrayList<Card>(player.getCardsIn(ZoneType.Hand)))game.getAction().moveTo(ZoneType.Library,c,null,new HashMap<>());
        Player cpu=game.getPlayers().get(1);
        for(int i=0;i<6;i++){place(human,"Island",ZoneType.Battlefield);place(human,"Mountain",ZoneType.Battlefield);place(cpu,"Plains",ZoneType.Battlefield);}
        for(String name:List.of("Goblin Piker","Storm Crow"))place(human,name,ZoneType.Battlefield);
        place(cpu,blockingFixture?"Siege Mastodon":"Savannah Lions",ZoneType.Battlefield);
        for(String name:List.of("Lightning Bolt","Shock","Unsummon","Opt","Counterspell","Island","Mountain"))place(human,name,ZoneType.Hand);
        for(int i=0;i<7;i++)place(cpu,"Plains",ZoneType.Hand);
        cpu.setLife(8,null);
        record("fixture-start",map("humanLife",human.getLife(),"cpuLife",cpu.getLife(),"setup","One-time initial zones; all later transitions belong to Forge"));
    }
    public static void main(String[] args)throws Exception {
        Path output=Path.of(args[4]);Files.createDirectories(output);
        ForgeWorker spike=new ForgeWorker(output);spike.focused=args.length>5 && !args[5].equals("realistic");spike.blockingFixture=args.length>5 && args[5].equals("blocking");
        spike.orderingFixture=args.length>5 && args[5].equals("ordering");
        spike.choicesFixture=args.length>5 && args[5].equals("choices");
        spike.verificationFixture=args.length>5 && args[5].equals("verification");
        try {spike.start(Path.of(args[0]),Path.of(args[1]),Integer.parseInt(args[2]),Path.of(args[3]));}
        catch(IllegalArgumentException error){Files.writeString(output.resolve("failure.json"),JSON.toJson(map("error",error.getMessage())));System.exit(2);}
    }
}
