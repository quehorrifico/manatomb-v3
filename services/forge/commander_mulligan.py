"""A single, audited rules correction on the retained Forge source."""
import hashlib


def extract(source, destination):
    original = source / "forge-game/src/main/java/forge/game/mulligan/MulliganService.java"
    raw = original.read_bytes()
    digest = hashlib.sha256(raw).hexdigest()
    # Refuse to patch a different upstream implementation silently.
    assert digest == "d4ced347709f261e0663edc964ca2e939cf60b0785f13bc2b6bef80724c21c90", "Pinned mulligan source changed"
    before = "game.getPlayers().size() > 2 || game.getRules().hasAppliedVariant(GameType.Brawl)"
    after = before + " || game.getRules().hasAppliedVariant(GameType.Commander)"
    text = raw.decode()
    assert text.count(before) == 1
    destination.write_text(text.replace(before, after))
    return {"source": str(original.relative_to(source)), "upstreamSha256": digest,
            "change": "First mulligan free in two-player Commander; all other mulligan logic retained"}
