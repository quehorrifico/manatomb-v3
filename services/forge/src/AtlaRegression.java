// Owner-reported sequence: two Eggs block the opposing commander and both die.
// Fixtures arrange the starting battlefield; combat and Atla's reveals are Forge's.
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
import forge.gamemodes.match.input.*;
import forge.player.*;

public final class AtlaRegression {
    static void check(boolean yes,String why){WorkerRegression.check(yes,why);}
    public static void main(String[] args)throws Exception {try{run(Path.of(args[2]));System.exit(0);}catch(Throwable e){e.printStackTrace();System.exit(1);}}
    static void run(Path out)throws Exception {
        ForgeWorker b=new ForgeWorker(out);GuiBase.setInterface(b.new Desktop());FModel.initialize(null,null);
        forge.util.MyRandom.setRandom(new Random(20261009));
        Path h=out.resolve("human.json"),c=out.resolve("cpu.json");
        Files.writeString(h,ForgeWorker.JSON.toJson(ForgeWorker.map("name","Atla fixture","commanders",List.of(ForgeWorker.map("name","Atla Palani, Nest Tender","quantity",1)),"main",List.of(ForgeWorker.map("name","Colossal Dreadmaw","quantity",1),ForgeWorker.map("name","Grizzly Bears","quantity",1),ForgeWorker.map("name","Forest","quantity",97)))));
        Files.writeString(c,ForgeWorker.JSON.toJson(ForgeWorker.map("name","Arahbo fixture","commanders",List.of(ForgeWorker.map("name","Arahbo, Roar of the World","quantity",1)),"main",List.of(ForgeWorker.map("name","Plains","quantity",99)))));
        Deck hd=b.readDeck(h),cd=b.readDeck(c);check(DeckFormat.Commander.getDeckConformanceProblem(hd)==null&&DeckFormat.Commander.getDeckConformanceProblem(cd)==null,"Illegal fixture deck");
        var hp=RegisteredPlayer.forCommander(hd).setPlayer(new LobbyPlayerHuman("Human"));var cp=RegisteredPlayer.forCommander(cd).setPlayer(GamePlayerUtil.createAiPlayer("CPU",0,"Default"));
        GameRules rules=new GameRules(GameType.Commander);rules.setAppliedVariants(EnumSet.of(GameType.Commander));Match match=new Match(rules,List.of(hp,cp),"Atla regression");
        b.game=match.createGame();b.human=b.game.getPlayers().get(0);b.controller=(PlayerControllerHuman)b.human.getController();
        b.gui=(IGuiGame)Proxy.newProxyInstance(IGuiGame.class.getClassLoader(),new Class[]{IGuiGame.class},b);b.controller.setGui(b.gui);
        b.controller.getInputQueue().addObserver((source,value)->{synchronized(b.gate){if(b.pending!=null&&b.pending.reply==null){b.pending=null;b.published=b.workingView();}}});b.game.subscribeToEvents(b);
        var finished=CompletableFuture.runAsync(()->{try{match.startGame(b.game,()->{
            for(Player player:b.game.getPlayers())for(Card commander:new ArrayList<Card>(player.getCardsIn(ZoneType.Command)))if(commander.getType().isCreature())b.game.getAction().moveTo(ZoneType.Battlefield,commander,null,new HashMap<>());
            for(Card card:new ArrayList<Card>(b.human.getCardsIn(ZoneType.Hand)))b.game.getAction().moveTo(ZoneType.Library,card,null,new HashMap<>());
            for(int i=0;i<2;i++){Card egg=CardFactory.getCard(forge.StaticData.instance().getAllTokens().getToken("g_0_1_egg_defender","C19"),b.human,b.game);b.game.getAction().moveTo(ZoneType.Battlefield,egg,null,new HashMap<>());}
        });}catch(Throwable failure){b.fail(failure);throw new CompletionException(failure);}});
        ForgeWorker.Pending previous=null;Set<Integer> chosenEggs=new HashSet<>();boolean ordered=false;List<String> labels=new ArrayList<>();
        for(int step=0;step<100;step++) {
            var p=DamageTriggerRegression.next(b,previous,finished);previous=p;
            System.out.println("Atla prompt "+p.kind+" at "+b.game.getPhaseHandler().getPhase());
            if(ordered && b.human.getCardsIn(ZoneType.Battlefield).stream().filter(card->Set.of("Colossal Dreadmaw","Grizzly Bears").contains(card.getName())).count()==2) {
                check(chosenEggs.size()==2,"Two Eggs were not declared as blockers");
                check(b.human.getCardsIn(ZoneType.Battlefield).stream().noneMatch(card->card.getType().hasSubtype("Egg")),"Eggs did not die");
                String log=ForgeWorker.JSON.toJson(b.publicLog);check(log.contains("blocked Arahbo")&&log.contains("damage to Egg")&&log.contains("Egg Token died")&&log.contains("entered the battlefield"),"Readable combat/death/reveal events missing: "+log);
                check(p.concede,"Expected stable priority after both Atla resolutions");var end=WorkerRegression.reply(b,p,"atla-end");end.addProperty("action","concede");b.accept(end);finished.get(10,TimeUnit.SECONDS);
                Files.writeString(out.resolve("checks.json"),ForgeWorker.JSON.toJson(ForgeWorker.map("passed",true,"twoEggsBlockedCommander",true,"bothEggsDied",true,"explicitTriggerOrder",labels,"bothCreaturesEntered",true,"readableCombatEvents",true)));
                System.out.println("PASS two Eggs block Arahbo, both die, distinct Atla triggers and both reveals resolve");return;
            }
            JsonObject action=WorkerRegression.reply(b,p,"atla-"+step);
            if(p.reply!=null) {
                if(p.kind.equals("orderSimultaneousAbilities")) {
                    check(p.options.size()==2,"Expected two Atla death triggers");Files.writeString(out.resolve("ordering-view.json"),b.published);
                    for(JsonElement option:JsonParser.parseString(b.published).getAsJsonObject().getAsJsonObject("prompt").getAsJsonArray("options"))labels.add(option.getAsJsonObject().get("label").getAsString());
                    check(new HashSet<>(labels).size()==2,"Indistinguishable Egg triggers");
                    check(labels.stream().allMatch(label->label.contains("Atla Palani")),"Wrong trigger source");
                    action.add("selected",JsonParser.parseString("[1,0]"));ordered=true;
                } else if(p.kind.equals("reveal")) { /* Explicit acknowledgement of Forge's reveal. */ }
                else if(p.kind.equals("getAbilityToPlay")&&p.options.size()==1) action.add("selected",JsonParser.parseString("[0]"));
                else throw new AssertionError("Unexpected explicit choice: "+p.kind+" "+b.published);
            } else if(p.input instanceof InputBlock && chosenEggs.size()<2) {
                var egg=p.cards.entrySet().stream().filter(e->e.getValue().getCurrentState().getType().hasSubtype("Egg")&&!chosenEggs.contains(e.getValue().getId())).findFirst().orElseThrow();
                chosenEggs.add(egg.getValue().getId());action.addProperty("action","card");action.addProperty("id",egg.getKey());
            } else if(p.input instanceof InputPassPriority) action.addProperty("action",b.okLabel.equals("Accept")?"cancel":"ok");
            else {check(p.ok,"No accepted OK action: "+p.kind);action.addProperty("action","ok");}
            b.accept(action);
        }
        throw new AssertionError("Atla fixture exceeded prompt bound");
    }
}
