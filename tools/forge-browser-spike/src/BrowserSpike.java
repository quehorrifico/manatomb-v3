// Disposable local adapter for the pinned Forge build. No gameplay rules are implemented here.
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
import forge.game.zone.ZoneType;
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

public final class BrowserSpike implements InvocationHandler {
    static final Gson JSON = new GsonBuilder().disableHtmlEscaping().create();
    final Object gate = new Object();
    final String incarnation = UUID.randomUUID().toString();
    final String csrf = UUID.randomUUID().toString();
    final Set<String> consumed = new HashSet<>();
    final Set<Integer> selectable = new HashSet<>(), highlighted = new HashSet<>();
    final List<Map<String,Object>> trace = new ArrayList<>();
    final Path output;
    final ThreadLocal<Set<Integer>> reveals=ThreadLocal.withInitial(HashSet::new);
    final ThreadLocal<Boolean> revealText=ThreadLocal.withInitial(()->false);
    volatile String published = "{\"status\":\"starting\",\"revision\":0}";
    Game game; Player human; PlayerControllerHuman controller; IGuiGame gui;
    long revision, paintSerial; Pending pending; Input shownInput, paintedInput;
    String promptText="", okLabel="OK", cancelLabel="Cancel";
    boolean okEnabled, cancelEnabled, invalidAction, actionInFlight;
    int port; boolean focused, blockingFixture, orderingFixture; volatile boolean blocked;
    String actionError="";
    static final class Pending {
        String id=UUID.randomUUID().toString(), kind;
        long revision; Input input; List<?> options; int min,max;
        CompletableFuture<List<Integer>> reply;
        Map<Integer,CardView> cards=new HashMap<>();
        Map<Integer,PlayerView> players=new HashMap<>();
        boolean ok,cancel;
    }
    BrowserSpike(Path output) { this.output=output; }
    static Map<String,Object> map(Object... pairs) {
        Map<String,Object> result=new LinkedHashMap<>();
        for(int i=0;i<pairs.length;i+=2) result.put((String)pairs[i],pairs[i+1]);
        return result;
    }
    void fail(Throwable failure) {
        failure.printStackTrace(); blocked=true;
        synchronized(gate) {
            pending=null;
            published=JSON.toJson(map("status","blocked","revision",++revision,"error","Unsupported or failed Forge path; see local diagnostic log"));
        }
    }
    final class Desktop extends GuiDesktop {
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
        GamePlayerUtil.getGuiPlayer().setName("Human");
        MyRandom.setRandom(new Random(20260912));
        Deck h=DeckSerializer.fromFile(deck1.toFile()), cpu=DeckSerializer.fromFile(deck2.toFile());
        for(Deck d:List.of(h,cpu)) {
            String problem=DeckFormat.Commander.getDeckConformanceProblem(d);
            if(problem!=null) throw new IllegalArgumentException(d.getName()+": "+problem);
        }
        var hp=RegisteredPlayer.forCommander(h).setPlayer(new LobbyPlayerHuman("Human"));
        var cp=RegisteredPlayer.forCommander(cpu).setPlayer(GamePlayerUtil.createAiPlayer("CPU",0,"Default"));
        GameRules rules=new GameRules(GameType.Commander); rules.setAppliedVariants(EnumSet.of(GameType.Commander));
        Match match=new Match(rules,List.of(hp,cp),"V2-011 local proof");
        game=match.createGame();
        human=game.getPlayers().get(0); controller=(PlayerControllerHuman)human.getController();
        gui=(IGuiGame)java.lang.reflect.Proxy.newProxyInstance(IGuiGame.class.getClassLoader(),new Class[]{IGuiGame.class},this);
        controller.setGui(gui);
        controller.getInputQueue().addObserver((source,value)->{
            synchronized(gate) { if(pending!=null && pending.reply==null) {pending=null; published=JSON.toJson(map("status","working","revision",++revision));} }
        });
        game.subscribeToEvents(this);
        HttpServer server=HttpServer.create(new InetSocketAddress("127.0.0.1",port),0);
        server.setExecutor(Executors.newFixedThreadPool(4));
        server.createContext("/", exchange->{
            try {
                if(!Objects.equals(exchange.getRequestHeaders().getFirst("Host"),"127.0.0.1:"+port)) {respond(exchange,403,"text/plain","Invalid host");return;}
                String path=exchange.getRequestURI().getPath();
                if(exchange.getRequestMethod().equals("GET") && path.equals("/")) respond(exchange,200,"text/html",Files.readString(html).replace("CSRF_TOKEN",csrf));
                else if(exchange.getRequestMethod().equals("GET") && path.equals("/state")) respond(exchange,200,"application/json",published);
                else if(exchange.getRequestMethod().equals("POST") && path.equals("/action")) {
                    if(!Objects.equals(exchange.getRequestHeaders().getFirst("Origin"),"http://127.0.0.1:"+port)) {respond(exchange,403,"text/plain","Invalid origin");return;}
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
            try {match.startGame(game,focused?this::setupFixture:null); synchronized(gate) {pending=null; published=JSON.toJson(snapshot("finished",null));} record("natural-result",map("turn",game.getPhaseHandler().getTurn()));}
            catch(Throwable failure) {fail(failure);}
        },"Game V2-011").start();
    }
    static void respond(HttpExchange exchange,int status,String type,String body) throws IOException {
        byte[] bytes=body.getBytes(StandardCharsets.UTF_8);exchange.getResponseHeaders().set("Content-Type",type+"; charset=utf-8");
        exchange.getResponseHeaders().set("Cache-Control","no-store");exchange.getResponseHeaders().set("X-Content-Type-Options","nosniff");
        exchange.sendResponseHeaders(status,bytes.length);try(var out=exchange.getResponseBody()){out.write(bytes);}
    }
    void record(String event,Map<String,Object> details) {
        synchronized(trace) {trace.add(map("event",event,"details",details));try {Files.writeString(output.resolve("transcript.json"),JSON.toJson(trace));} catch(IOException e){throw new UncheckedIOException(e);}}
    }
    boolean visible(CardView c) {return c!=null && (reveals.get().contains(c.getId()) || c.canBeShownTo(human.getView()) && c.canFaceDownBeShownTo(human.getView()));}
    String safeText(String text) {
        if(text==null)return "";
        // Raw logs/objects never cross the boundary. Additionally scrub hidden identities from prompt prose.
        Set<String> hidden=new TreeSet<>(Comparator.comparingInt(String::length).reversed().thenComparing(String::compareTo));
        for(Player p:game.getPlayers()) for(ZoneType z:ZoneType.values()) for(Card c:p.getCardsIn(z)) if(!visible(c.getView())) {
            hidden.add(c.getName());
            if(c.getView().getAlternateState()!=null) hidden.add(c.getView().getAlternateState().getName());
        }
        // A hidden copy must not censor an already authorized identity (e.g. basic lands).
        for(Player p:game.getPlayers())for(ZoneType z:ZoneType.values())for(Card c:p.getCardsIn(z))if(visible(c.getView()))hidden.remove(c.getName());
        for(String name:hidden) if(name!=null&&!name.isBlank()) text=text.replace(name,"[hidden card]");
        return text;
    }
    Map<String,Object> card(CardView cv, Pending p) {
        boolean show=visible(cv);
        Map<String,Object> dto=map("name",show?cv.getCurrentState().getName():"Face-down card","tapped",cv.isTapped(),"attacking",cv.isAttacking(),"blocking",cv.isBlocking());
        if(show) {
            dto.put("id",cv.getId());dto.put("mana",cv.getCurrentState().getManaCost().toString());
            dto.put("type",cv.getCurrentState().getType().toString());
            dto.put("pt",cv.getCurrentState().getPower()+"/"+cv.getCurrentState().getToughness());
            dto.put("selected",highlighted.contains(cv.getId()));
            if(p!=null) p.cards.put(cv.getId(),cv);
        }
        return dto;
    }
    Map<String,Object> snapshot(String status,Pending p) {
        List<Object> players=new ArrayList<>();
        for(Player player:game.getPlayers()) {
            List<Object> zones=new ArrayList<>();
            for(ZoneType zone:List.of(ZoneType.Hand,ZoneType.Battlefield,ZoneType.Graveyard,ZoneType.Exile,ZoneType.Command,ZoneType.Library)) {
                var cards=player.getCardsIn(zone); List<Object> shown=new ArrayList<>();
                for(Card c:cards) if(visible(c.getView()) || (zone==ZoneType.Battlefield && c.isFaceDown())) shown.add(card(c.getView(),p));
                // Never export hidden IDs or ordering; visible subsets sorted by public name/id.
                if(zone==ZoneType.Library || zone==ZoneType.Hand && player!=human) shown.sort(Comparator.comparing(Object::toString));
                zones.add(map("zone",zone.name(),"count",cards.size(),"cards",shown));
            }
            if(p!=null)p.players.put(player.getView().getId(),player.getView());
            players.add(map("id",player.getView().getId(),"name",player==human?"Human":"CPU","life",player.getLife(),"zones",zones));
        }
        List<Object> stack=new ArrayList<>();
        if(game.getView().getStack()!=null) for(StackItemView item:game.getView().getStack()) {
            CardView source=item.getSourceCard();stack.add(map("source",visible(source)?source.getCurrentState().getName():"Hidden source"));
        }
        Map<String,Object> dto=map("status",status,"session",incarnation,"revision",++revision,"players",players,"stack",stack,"turn",game.getPhaseHandler().getTurn(),"phase",String.valueOf(game.getPhaseHandler().getPhase()),"turnPlayer",game.getPhaseHandler().getPlayerTurn()==human?"Human":"CPU");
        if(status.equals("finished")) dto.put("result",game.getOutcome().isDraw()?"Draw":game.getOutcome().getWinningLobbyPlayer().getName());
        return dto;
    }
    void publishInput() {
        if(blocked || controller==null || game.isGameOver())return;
        Input input=controller.getInputQueue().getInput();
        if(input==null || paintedInput!=input || controller.getInputProxy().getInput()!=input || input instanceof InputPayMana pay && pay.isActivatingManaAbility()) return;
        synchronized(gate) {
            if(actionInFlight || pending!=null && pending.reply!=null)return;
            // Only called after a complete UI dispatch. No HTTP thread reads mutable Forge state.
            Pending p=new Pending();p.kind=input.getClass().getSimpleName();p.input=input;
            p.ok=okEnabled && !okLabel.equals("Auto");p.cancel=cancelEnabled && !cancelLabel.startsWith("Undo") && !cancelLabel.equals("Auto");
            Map<String,Object> dto=snapshot("input",p);p.revision=revision;
            dto.put("prompt",map("id",p.id,"kind",p.kind,"text",safeText(promptText),"ok",okLabel,"cancel",cancelLabel,"okEnabled",p.ok,"cancelEnabled",p.cancel));
            dto.put("actionError",actionError);pending=p;shownInput=input;published=JSON.toJson(dto);
            record("projection",dto);
            record("input",map("revision",revision,"kind",p.kind,"turn",game.getPhaseHandler().getTurn()));
        }
    }
    String label(Object option) {
        if(option instanceof CardView c)return visible(c)?c.getCurrentState().getName():"Hidden card";
        if(option instanceof SpellAbilityView a)return abilityLabel(a);
        if(option instanceof PlayerView p)return p.equals(human.getView())?"Human":"CPU";
        if(option instanceof forge.item.PaperCard c)return c.getName();
        if(option instanceof String || option instanceof Number || option instanceof Enum<?>)return revealText.get()?option.toString():safeText(option.toString());
        throw new UnsupportedOperationException("Unmapped choice type "+option.getClass().getName());
    }
    String abilityLabel(SpellAbilityView ability) {
        CardView source=ability.getHostCard();
        if(!visible(source))return "Hidden ability";
        String raw=Objects.toString(ability.getDescription(), "");
        String description=safeText(raw.replaceFirst(" \\[(?:Card|ZoneChanger|Activator):.*", "")).trim();
        // A secondary trigger can have no prose upstream, only a stack annotation.
        // Use the authorized current face's card text as context, never infer a rule.
        boolean missing=description.replaceAll("\\[[^\\]]*\\]", "").isBlank();
        String result=source.getCurrentState().getName()+" — "+description;
        if(missing) {
            String rules=safeText(source.getCurrentState().getOracleText());
            if(rules.isBlank())throw new UnsupportedOperationException("Ability lacks authorized explanatory text");
            result+=" · Source rules: "+rules;
            record("ability-context",map("source",source.getCurrentState().getName(),
                "viewDescription",safeText(raw),"translatedDescription",description,
                "scrubChangedDescription",!raw.equals(safeText(raw)),
                "metadataRemoved",!raw.equals(raw.replaceFirst(" \\[(?:Card|ZoneChanger|Activator):.*", "")),
                "context","Current visible face's Forge Oracle text"));
        }
        return result;
    }
    List<Integer> ask(String kind,String message,List<?> options,int min,int max) throws Exception {
        Pending p=new Pending();p.kind=kind;p.options=List.copyOf(options);p.min=min;p.max=max;p.reply=new CompletableFuture<>();
        synchronized(gate) {
            Map<String,Object> dto=snapshot("choice",p);p.revision=revision;
            List<Object> opts=new ArrayList<>();Set<String> abilityLabels=new HashSet<>();
            for(int i=0;i<options.size();i++) {
                String text=label(options.get(i));
                if(options.get(i) instanceof SpellAbilityView && !abilityLabels.add(text))
                    throw new UnsupportedOperationException("Indistinguishable ability options need additional authorized context");
                opts.add(map("id",i,"label",text));
            }
            dto.put("prompt",map("id",p.id,"kind",kind,"text",safeText(message),"min",min,"max",max,"options",opts,"ordering",kind.equals("orderSimultaneousAbilities")));
            pending=p;published=JSON.toJson(dto);record("projection",dto);record("choice",map("kind",kind,"revision",revision,"thread",Thread.currentThread().getName()));
        }
        List<Integer> result=p.reply.get();
        record("choice-unblocked",map("kind",kind,"thread",Thread.currentThread().getName()));
        return result;
    }
    Object orderSimultaneous(Object[] a) throws Exception {
        // Pin-specific boundary: no generic card/sideboard/replacement ordering support.
        boolean caller=StackWalker.getInstance().walk(frames->frames.anyMatch(f->
            f.getClassName().equals(PlayerControllerHuman.class.getName()) && f.getMethodName().equals("orderSimultaneousSa")));
        if(!caller || a.length!=9 || !Integer.valueOf(0).equals(a[2]) || !Integer.valueOf(0).equals(a[3])
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
    Object one(String kind,String text,List<?> options,boolean optional) throws Exception {
        var selected=ask(kind,text,options,optional?0:1,1);return selected.isEmpty()?null:options.get(selected.get(0));
    }
    String accept(JsonObject body) {
        if(!body.has("csrf") || !csrf.equals(body.get("csrf").getAsString()))throw new IllegalArgumentException("Invalid token");
        String request=body.get("request").getAsString(),action=body.get("action").getAsString();
        Pending p;
        synchronized(gate) {
            if(consumed.contains(request))throw new IllegalArgumentException("Repeated request");
            p=pending;
            if(p==null || !incarnation.equals(body.get("session").getAsString()) || p.revision!=body.get("revision").getAsLong() || !p.id.equals(body.get("prompt").getAsString()))throw new IllegalArgumentException("Stale prompt or revision");
            if(p.reply!=null) {
                if(!action.equals("reply"))throw new IllegalArgumentException("Expected reply");
                List<Integer> selected=new ArrayList<>();
                if(p.kind.equals("orderSimultaneousAbilities")) {
                    if(!body.has("selected") || !body.get("selected").isJsonArray() || body.has("rememberDecision"))throw new IllegalArgumentException("Expected an explicit order without remembering");
                    for(JsonElement e:body.getAsJsonArray("selected")) {
                        if(!e.isJsonPrimitive() || !e.getAsJsonPrimitive().isNumber() || !e.getAsString().matches("0|[1-9][0-9]*"))throw new IllegalArgumentException("Invalid order entry");
                        try {selected.add(Integer.parseInt(e.getAsString()));}catch(NumberFormatException failure){throw new IllegalArgumentException("Invalid order entry");}
                    }
                } else for(JsonElement e:body.getAsJsonArray("selected"))selected.add(e.getAsInt());
                if(selected.size()<p.min||selected.size()>p.max||new HashSet<>(selected).size()!=selected.size()||selected.stream().anyMatch(i->i<0||i>=p.options.size()))throw new IllegalArgumentException("Invalid choice");
                consumed.add(request);pending=null;published=JSON.toJson(map("status","working","revision",++revision));
                record("browser-reply",map("kind",p.kind,"selected",selected));p.reply.complete(selected);
            } else {
                if(!Set.of("ok","cancel","card","player").contains(action))throw new IllegalArgumentException("Invalid action");
                if(action.equals("ok")&&!p.ok || action.equals("cancel")&&!p.cancel)throw new IllegalArgumentException("Disabled control");
                int id=body.has("id")?body.get("id").getAsInt():-1;
                if(action.equals("card")&&!p.cards.containsKey(id) || action.equals("player")&&!p.players.containsKey(id))throw new IllegalArgumentException("Unoffered entity");
                if(p.kind.equals("InputConfirmMulligan") && action.equals("card"))throw new IllegalArgumentException("Mulligan card abilities not supported by this spike");
                consumed.add(request);pending=null;published=JSON.toJson(map("status","working","revision",++revision));
                actionInFlight=true;
                SwingUtilities.invokeLater(()->{
                    try {
                        if(controller.getInputQueue().getInput()!=p.input || controller.getInputProxy().getInput()!=p.input) throw new IllegalArgumentException("Input advanced before dispatch");
                        invalidAction=false;actionError="";
                        boolean accepted=true;
                        switch(action) {
                            case "ok" -> controller.selectButtonOk();
                            case "cancel" -> controller.selectButtonCancel();
                            case "card" -> accepted=controller.selectCard(p.cards.get(id),null,null);
                            case "player" -> controller.selectPlayer(p.players.get(id),null);
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
            case "toString":return "BrowserSpikeGui";
            case "hashCode":return System.identityHashCode(proxy);
            case "equals":return proxy==a[0];
            case "getGameView":return game.getView();
            case "isLibgdxPort", "isNetGame", "isGamePaused", "isUiSetToSkipPhase":return false;
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
            case "getAbilityToPlay":return one(name,"Choose an ability",(List<?>)a[1],true);
            case "showConfirmDialog":return one(name,(String)a[0],List.of(a[2],a[3]),false).equals(a[2]);
            case "confirm":return one(name,(String)a[1],(List<?>)a[3],false).equals(((List<?>)a[3]).get(0));
            case "one":return one(name,(String)a[0],(List<?>)a[1],false);
            case "oneOrNone":return one(name,(String)a[0],(List<?>)a[1],true);
            case "manipulateCardList": {
                List<CardView> all=new ArrayList<>(),movable=new ArrayList<>();
                for(Object item:(Iterable<?>)a[1])all.add((CardView)item);
                for(Object item:(Iterable<?>)a[2])movable.add((CardView)item);
                if(movable.size()!=1 || (Boolean)a[5])throw new UnsupportedOperationException("Multiple-card/arbitrary ordering is not mapped");
                List<String> locations=new ArrayList<>();if((Boolean)a[3])locations.add("Top");if((Boolean)a[4])locations.add("Bottom");
                String selected=(String)one(name,"Place "+label(movable.get(0)),locations,false);
                all.remove(movable.get(0));if(selected.equals("Top"))all.add(0,movable.get(0));else all.add(movable.get(0));
                return all;
            }
            case "chooseSingleEntityForEffect":return one(name,(String)a[0],(List<?>)a[1],(Boolean)a[3]);
            case "getInteger": {
                int min=(Integer)a[1],max=(Integer)a[2];if(max-min>200)throw new UnsupportedOperationException("Large integer range");
                List<Integer> choices=new ArrayList<>();for(int i=min;i<=max;i++)choices.add(i);return one(name,(String)a[0],choices,false);
            }
            case "getChoices": {
                List<?> options=(List<?>)a[3];List<Object> out=new ArrayList<>();for(int i:ask(name,(String)a[0],options,(Integer)a[1],(Integer)a[2]))out.add(options.get(i));return out;
            }
            case "showOptionDialog":return ask(name,(String)a[0],(List<?>)a[3],1,1).get(0);
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
    void setupFixture() {
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
        BrowserSpike spike=new BrowserSpike(output);spike.focused=args.length>5 && !args[5].equals("realistic");spike.blockingFixture=args.length>5 && args[5].equals("blocking");
        spike.orderingFixture=args.length>5 && args[5].equals("ordering");
        spike.start(Path.of(args[0]),Path.of(args[1]),Integer.parseInt(args[2]),Path.of(args[3]));
    }
}
