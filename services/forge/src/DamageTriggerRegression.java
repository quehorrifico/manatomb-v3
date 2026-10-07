// Exact owner-reported sequence through the pinned human controller and turn engine.
// One initial fixture arrangement; all attack, reveal, damage and Treasure rules are Forge's.
import java.nio.file.*;
import java.util.*;
import java.util.concurrent.*;
import java.lang.reflect.Proxy;
import com.google.gson.*;
import forge.gui.GuiBase;
import forge.gui.interfaces.IGuiGame;
import forge.model.FModel;
import forge.deck.*;
import forge.game.*;
import forge.game.card.*;
import forge.game.player.*;
import forge.game.zone.ZoneType;
import forge.game.phase.PhaseType;
import forge.gamemodes.match.input.*;
import forge.player.*;

public final class DamageTriggerRegression {
    static final String COMMANDER="Raph & Mikey, Troublemakers", DRAGON="Old Gnawbone";
    static void check(boolean yes,String why){WorkerRegression.check(yes,why);}
    public static void main(String[] args)throws Exception {try{run(Path.of(args[2]));System.exit(0);}catch(Throwable e){e.printStackTrace();System.exit(1);}}
    static ForgeWorker.Pending next(ForgeWorker b,ForgeWorker.Pending previous,CompletableFuture<Void> game)throws Exception {
        for(int i=0;i<1000;i++){
            synchronized(b.gate){if(b.blocked)throw new AssertionError("Exact combat sequence blocked: "+b.published);if(b.pending!=null&&b.pending!=previous)return b.pending;}
            if(game.isCompletedExceptionally())game.get();Thread.sleep(10);
        }
        throw new AssertionError("No next prompt after "+(previous==null?"start":previous.kind));
    }
    static void run(Path out)throws Exception {
        ForgeWorker b=new ForgeWorker(out);GuiBase.setInterface(b.new Desktop());FModel.initialize(null,null);
        forge.util.MyRandom.setRandom(new Random(20260912));
        Path h=out.resolve("fixture-human.json"),c=out.resolve("fixture-cpu.json");
        Files.writeString(h,ForgeWorker.JSON.toJson(ForgeWorker.map("name","Damage trigger fixture","commanders",List.of(ForgeWorker.map("name",COMMANDER,"quantity",1)),"main",List.of(ForgeWorker.map("name",DRAGON,"quantity",1),ForgeWorker.map("name","Forest","quantity",98)))));
        Files.writeString(c,ForgeWorker.JSON.toJson(ForgeWorker.map("name","Nonblocking fixture","commanders",List.of(ForgeWorker.map("name","Progenitus","quantity",1)),"main",List.of(ForgeWorker.map("name","Plains","quantity",99)))));
        Deck hd=b.readDeck(h),cd=b.readDeck(c);check(DeckFormat.Commander.getDeckConformanceProblem(hd)==null&&DeckFormat.Commander.getDeckConformanceProblem(cd)==null,"Illegal fixture deck");
        var hp=RegisteredPlayer.forCommander(hd).setPlayer(new LobbyPlayerHuman("Human"));var cp=RegisteredPlayer.forCommander(cd).setPlayer(GamePlayerUtil.createAiPlayer("CPU",0,"Default"));
        GameRules rules=new GameRules(GameType.Commander);rules.setAppliedVariants(EnumSet.of(GameType.Commander));Match match=new Match(rules,List.of(hp,cp),"Damage trigger regression");
        b.game=match.createGame();b.human=b.game.getPlayers().get(0);b.controller=(PlayerControllerHuman)b.human.getController();
        b.gui=(IGuiGame)Proxy.newProxyInstance(IGuiGame.class.getClassLoader(),new Class[]{IGuiGame.class},b);b.controller.setGui(b.gui);
        b.controller.getInputQueue().addObserver((source,value)->{synchronized(b.gate){if(b.pending!=null&&b.pending.reply==null){b.pending=null;b.published=b.workingView();}}});b.game.subscribeToEvents(b);
        var finished=CompletableFuture.runAsync(()->{try{match.startGame(b.game,()->{
            for(Card card:new ArrayList<Card>(b.human.getCardsIn(ZoneType.Hand)))b.game.getAction().moveTo(ZoneType.Library,card,null,new HashMap<>());
            Card commander=b.human.getCardsIn(ZoneType.Command).stream().filter(card->card.getName().equals(COMMANDER)).findFirst().orElseThrow();
            b.game.getAction().moveTo(ZoneType.Battlefield,commander,null,new HashMap<>());
            Card dragon=b.human.getCardsIn(ZoneType.Library).stream().filter(card->card.getName().equals(DRAGON)).findFirst().orElseThrow();b.game.getAction().moveToLibrary(dragon,3,null);
        });}catch(Throwable failure){b.fail(failure);throw new CompletionException(failure);}});
        ForgeWorker.Pending previous=null;boolean declared=false,revealed=false,enteredAttacking=false,ordered=false;List<String> damageLabels=new ArrayList<>();int step=0;
        for(;step<75;step++) {
            var p=next(b,previous,finished);previous=p;
            System.out.println("Fixture prompt "+p.kind+" at "+b.game.getPhaseHandler().getPhase());
            Card dragon=b.human.getCardsIn(ZoneType.Battlefield).stream().filter(card->card.getName().equals(DRAGON)).findFirst().orElse(null);
            if(dragon!=null&&b.game.getCombat()!=null&&b.game.getCombat().isAttacking(dragon))enteredAttacking=dragon.isTapped();
            long treasure=b.human.getCardsIn(ZoneType.Battlefield).stream().filter(card->card.isToken()&&card.getType().hasSubtype("Treasure")).count();
            if(treasure==14) {
                check(declared&&revealed&&enteredAttacking&&ordered,"Exact sequence was bypassed");check(b.game.getPlayers().get(1).getLife()==26,"Unexpected combat damage");
                Files.writeString(out.resolve("checks.json"),ForgeWorker.JSON.toJson(ForgeWorker.map("passed",true,"commander",COMMANDER,"revealedIntoAttack",DRAGON,"actualHumanAttackDeclaration",true,"actualDamageAndTreasureResolution",true,"cpuLife",26,"treasureCount",treasure,"orderedTriggerLabels",damageLabels,"unequalDamageLabels",true,"hiddenAndUnknownContextRejected",true,"promptCount",step)));
                check(p.concede,"Expected stable priority after resolution");var end=WorkerRegression.reply(b,p,"fixture-concede");end.addProperty("action","concede");b.accept(end);finished.get(10,TimeUnit.SECONDS);System.out.println("PASS exact Raph & Mikey attack reveal, Old Gnawbone damage triggers, explicit ordering and fourteen Treasures");return;
            }
            JsonObject action=WorkerRegression.reply(b,p,"damage-fixture-"+step);
            if(p.reply!=null) {
                if(p.kind.equals("getAbilityToPlay")) {
                    Files.writeString(out.resolve("ability-view.json"),b.published);
                    check(p.options.size()==1&&p.options.get(0) instanceof forge.game.spellability.SpellAbilityView&&Set.of(COMMANDER,DRAGON).contains(((forge.game.spellability.SpellAbilityView)p.options.get(0)).getHostCard().getName())&&!((forge.game.spellability.SpellAbilityView)p.options.get(0)).isSpell(),"Unexpected offered ability");
                    action.add("selected",JsonParser.parseString("[0]"));
                }
                else if(p.kind.equals("reveal")){check(b.published.contains(DRAGON),"Expected commander reveal");revealed=true;}
                else if(p.kind.equals("orderSimultaneousAbilities")) {
                    check(p.options.size()==2,"Expected both Old Gnawbone damage triggers");Files.writeString(out.resolve("ordering-view.json"),b.published);JsonObject view=JsonParser.parseString(b.published).getAsJsonObject();
                    for(JsonElement option:view.getAsJsonObject("prompt").getAsJsonArray("options"))damageLabels.add(option.getAsJsonObject().get("label").getAsString());
                    check(new HashSet<>(damageLabels).size()==2,"Damage trigger labels are identical");check(damageLabels.stream().anyMatch(label->label.contains(COMMANDER))&&damageLabels.stream().allMatch(label->label.contains(DRAGON)),"Missing visible damaging creature context");
                    var loc=forge.util.Localizer.getInstance();
                    Card commander=b.human.getCardsIn(ZoneType.Battlefield).stream().filter(card->card.getName().equals(COMMANDER)).findFirst().orElseThrow();
                    String prefix=" ["+loc.getMessage("lblDamageSource")+": ",suffix=", "+loc.getMessage("lblDamaged")+": "+b.game.getPlayers().get(1)+", "+loc.getMessage("lblAmount")+": ";
                    String seven=prefix+commander+suffix+"7]",three=prefix+commander+suffix+"3]";
                    check(b.publicDamageContext(seven).endsWith(": 7")&&b.publicDamageContext(three).endsWith(": 3")&&!b.publicDamageContext(seven).equals(b.publicDamageContext(three)),"Unequal public damage amounts were merged");
                    Card hidden=b.game.getPlayers().get(1).getCardsIn(ZoneType.Hand).getFirst();
                    check(!b.visible(hidden.getView())&&b.publicDamageContext(prefix+hidden+suffix+"7]").isEmpty(),"Hidden damage source leaked");
                    check(b.publicDamageContext(prefix+"Unknown entity (999999)"+suffix+"7]").isEmpty(),"Unknown engine reference accepted");
                    check(b.publicDamageContext(prefix+commander+", "+loc.getMessage("lblDamaged")+": "+hidden+", "+loc.getMessage("lblAmount")+": 7]").isEmpty(),"Hidden damage target leaked");
                    check(b.publicDamageContext(" [Unrecognized: "+commander+", Amount: 7]").isEmpty(),"Unrecognized annotation accepted");
                    check(damageLabels.stream().noneMatch(label->label.contains(commander.toString())),"Engine card ID leaked into public labels");
                    action.add("selected",JsonParser.parseString("[1,0]"));ordered=true;
                } else throw new AssertionError("Unexpected choice requiring explicit fixture decision: "+p.kind);
            } else if(p.input instanceof InputAttack) {
                if(!declared){int id=p.cards.entrySet().stream().filter(e->e.getValue().getName().equals(COMMANDER)).map(Map.Entry::getKey).findFirst().orElseThrow();action.addProperty("action","card");action.addProperty("id",id);declared=true;}
                else {check(b.game.getCombat().getAttackers().size()==1,"Commander attack was not selected");action.addProperty("action","ok");}
            } else if(p.input instanceof InputPassPriority) {
                action.addProperty("action",b.okLabel.equals("Accept")?"cancel":"ok");
            } else {check(p.ok&&Set.of("Play","Keep").contains(b.okLabel),"Unexpected input requiring explicit fixture decision: "+p.kind);action.addProperty("action","ok");}
            b.accept(action);
        }
        throw new AssertionError("Exact combat fixture exceeded prompt bound");
    }
}
