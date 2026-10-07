"""Extract pinned Forge desktop assignment validation, without Swing presentation.
The two rule-bearing method bodies are retained verbatim, except the poison-
counter view accessor is supplied as a scalar instead of through CMatchUI.
No card-specific logic is added. GPL-3.0-or-later (Forge Team).
"""
from pathlib import Path
import hashlib
EXPECTED=''
def extract(source,output):
    p=source/'forge-gui-desktop/src/main/java/forge/screens/match/VAssignCombatDamage.java'
    text=p.read_text()
    def method(name):
        start=text.index('    private '+('void' if name=='checkDamageQueue' else 'int')+' '+name+'(')
        cursor=text.index('{',start);depth=1;end=cursor+1
        while depth:
            if text[end]=='{':depth+=1
            elif text[end]=='}':depth-=1
            end+=1
        return text[start:end]
    body=method('checkDamageQueue')+'\n'+method('getDamageToKill')
    body=body.replace('matchUI.getGameView().getPoisonCountersToLose()','poisonCountersToLose')
    license=text[:text.index('package ')]
    out=license+'''// Generated from pinned Forge VAssignCombatDamage; do not hand-edit rule bodies.
import java.util.*;
import forge.game.GameEntityView;
import forge.game.card.*;
import forge.game.player.PlayerView;
final class PinnedCombatAssignment {
    private boolean attackerHasDeathtouch,attackerHasDivideDamage,attackerHasInfect,overrideCombatantOrder;
    private GameEntityView defender;
    private int poisonCountersToLose;
    private static class DamageTarget { CardView card; int damage; DamageTarget(CardView c,int d){card=c;damage=d;} }
    private final List<DamageTarget> defenders=new ArrayList<>();
    static List<CardView> targets(CardView attacker,List<CardView> blockers,GameEntityView defender,boolean overrideOrder) {
        List<CardView> targets=new ArrayList<>(blockers);
        if(defender!=null && (attacker.getCurrentState().hasTrample() || attacker.getCurrentState().hasDivideDamage() && overrideOrder))targets.add(null);
        return targets;
    }
    static void validate(CardView attacker,List<CardView> targets,GameEntityView defender,boolean overrideOrder,int total,int poisonLimit,List<Integer> amounts) {
        if(amounts.size()!=targets.size() || amounts.stream().anyMatch(x->x<0) || amounts.stream().mapToLong(Integer::longValue).sum()!=total)throw new IllegalArgumentException("Assign exactly the offered damage total");
        PinnedCombatAssignment m=new PinnedCombatAssignment();
        m.attackerHasDeathtouch=attacker.getCurrentState().hasDeathtouch();m.attackerHasDivideDamage=attacker.getCurrentState().hasDivideDamage();m.attackerHasInfect=attacker.getCurrentState().hasInfect();m.overrideCombatantOrder=overrideOrder;m.defender=defender;m.poisonCountersToLose=poisonLimit;
        for(int i=0;i<targets.size();i++)m.defenders.add(new DamageTarget(targets.get(i),amounts.get(i)));
        m.checkDamageQueue();
        for(int i=0;i<amounts.size();i++)if(m.defenders.get(i).damage!=amounts.get(i))throw new IllegalArgumentException("Assignment does not satisfy Forge combat damage order/lethal constraints");
    }
'''+body+'\n}\n'
    output.write_text(out)
    return {'source':str(p.relative_to(source)),'source_sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'generated_sha256':hashlib.sha256(out.encode()).hexdigest(),'adaptation':'headless target list and allocation shape; pinned checkDamageQueue/getDamageToKill retained, CMatchUI poison count dependency injected'}
