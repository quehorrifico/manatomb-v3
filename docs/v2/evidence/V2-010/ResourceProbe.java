import forge.GuiDesktop;
import forge.gui.GuiBase;
import forge.model.FModel;
import forge.localinstance.properties.ForgeConstants;
import forge.localinstance.properties.ForgePreferences.FPref;
import forge.deck.Deck;
import forge.deck.DeckFormat;
import forge.deck.io.DeckSerializer;
import forge.game.player.RegisteredPlayer;
import java.io.File;

/** Read-only V2-010 catalog/deck probe; no game controller or bridge. */
public class ResourceProbe {
    public static void main(String[] args) {
        GuiBase.setInterface(new GuiDesktop());
        FModel.initialize(null, null);
        int editions = 0;
        for (var edition : FModel.getMagicDb().getEditions()) editions++;
        System.out.println("PROBE headless=" + java.awt.GraphicsEnvironment.isHeadless());
        System.out.println("PROBE editions=" + editions);
        System.out.println("PROBE uniqueCommonCards=" + FModel.getMagicDb().getCommonCards().getUniqueCards().size());
        System.out.println("PROBE lazy=" + FModel.getPreferences().getPref(FPref.LOAD_CARD_SCRIPTS_LAZILY));
        System.out.println("PROBE deckgen=" + FModel.getPreferences().getPref(FPref.DECKGEN_CARDBASED));
        System.out.println("PROBE mulligan=" + FModel.getPreferences().getPref(FPref.MULLIGAN_RULE));
        for (String name : args) {
            Deck deck = DeckSerializer.fromFile(new File(ForgeConstants.DECK_COMMANDER_DIR + name));
            String problem = DeckFormat.Commander.getDeckConformanceProblem(deck);
            System.out.println("PROBE deck=" + name + " main=" + deck.getMain().countAll()
                + " commanders=" + deck.getCommanders() + " startingLife=" + RegisteredPlayer.forCommander(deck).getStartingLife()
                + " conformance=" + (problem == null ? "PASS" : problem));
            if (problem != null) System.exit(2);
        }
        System.exit(0);
    }
}
