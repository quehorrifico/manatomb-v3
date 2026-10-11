// Exercise the corrected pinned service and its London mulligan implementation.
import java.nio.file.*;
import java.util.*;
import java.lang.reflect.*;
import forge.gui.GuiBase;
import forge.model.FModel;
import forge.deck.io.DeckSerializer;
import forge.game.*;
import forge.game.player.*;
import forge.game.mulligan.*;
import forge.player.*;

public final class MulliganRegression {
    public static void main(String[] args)throws Exception {try{run(args);System.exit(0);}catch(Throwable e){e.printStackTrace();System.exit(1);}}
    static void run(String[] args)throws Exception {
        ForgeWorker b=new ForgeWorker(Path.of(args[2]));GuiBase.setInterface(b.new Desktop());FModel.initialize(null,null);
        var hd=DeckSerializer.fromFile(Path.of(args[0]).toFile());var cd=DeckSerializer.fromFile(Path.of(args[1]).toFile());
        for(GameType type:List.of(GameType.Commander,GameType.Brawl,GameType.Constructed)) {
            var hp=RegisteredPlayer.forCommander(hd).setPlayer(new LobbyPlayerHuman("Human"));var cp=RegisteredPlayer.forCommander(cd).setPlayer(GamePlayerUtil.createAiPlayer("CPU",0,"Default"));
            GameRules rules=new GameRules(type);rules.setAppliedVariants(type==GameType.Constructed?EnumSet.noneOf(GameType.class):EnumSet.of(type));
            var game=new Match(rules,List.of(hp,cp),"Mulligan regression").createGame();
            var service=new MulliganService(game.getPlayers().get(0));Method init=MulliganService.class.getDeclaredMethod("initializeMulligans");init.setAccessible(true);init.invoke(service);
            Field list=MulliganService.class.getDeclaredField("mulligans");list.setAccessible(true);var mulligans=(List<?>)list.get(service);WorkerRegression.check(mulligans.size()==2,"Missing player mulligans");
            Field count=AbstractMulligan.class.getDeclaredField("timesMulliganed");count.setAccessible(true);
            for(Object value:mulligans){var mulligan=(AbstractMulligan)value;WorkerRegression.check(mulligan instanceof LondonMulligan,"Expected pinned London rule");
                WorkerRegression.check(mulligan.tuckCardsDuringMulligan()==0,"Initial hand costs a card");
                count.setInt(mulligan,1);WorkerRegression.check(mulligan.tuckCardsDuringMulligan()==(type==GameType.Constructed?1:0),"Wrong first mulligan for "+type);
                count.setInt(mulligan,2);WorkerRegression.check(mulligan.tuckCardsDuringMulligan()==(type==GameType.Constructed?2:1),"Wrong second mulligan for "+type);
            }
        }
        Files.writeString(Path.of(args[2]).resolve("checks.json"),"{\"passed\":true,\"commanderBothSeats\":true,\"firstFreeSecondCostsOne\":true,\"brawlUnchanged\":true,\"constructedUnchanged\":true}");
        System.out.println("PASS both Commander seats: first mulligan free, second costs one; Brawl and Constructed unchanged");
    }
}
