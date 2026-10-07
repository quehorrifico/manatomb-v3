// Actual pinned game/controller termination; distinct from browser coverage.
import java.nio.file.*;
import java.util.*;
import com.google.gson.*;

public final class ConcedeRegression {
    public static void main(String[] args) throws Exception {
        try {run(Path.of(args[2]));System.exit(0);} catch(Throwable e){e.printStackTrace();System.exit(1);}
    }
    static void run(Path out)throws Exception {
        Path human=out.resolve("human.json"),cpu=out.resolve("cpu.json");
        Files.writeString(human,ForgeWorker.JSON.toJson(ForgeWorker.map("name","Concession human","commanders",List.of(ForgeWorker.map("name","Silvos, Rogue Elemental","quantity",1)),"main",List.of(ForgeWorker.map("name","Forest","quantity",99)))));
        Files.writeString(cpu,ForgeWorker.JSON.toJson(ForgeWorker.map("name","Concession CPU","commanders",List.of(ForgeWorker.map("name","Isamaru, Hound of Konda","quantity",1)),"main",List.of(ForgeWorker.map("name","Plains","quantity",99)))));
        ForgeWorker b=new ForgeWorker(out);b.start(human,cpu,0,Path.of("/dev/null"));
        ForgeWorker.Pending previous=null;
        for(int step=0;step<6;step++) {
            ForgeWorker.Pending p=null;
            for(int i=0;i<1000;i++){synchronized(b.gate){if(b.pending!=null&&b.pending!=previous){p=b.pending;break;}}Thread.sleep(10);}
            WorkerRegression.check(p!=null,"No next input");
            JsonObject action=WorkerRegression.reply(b,p,"concede-test-"+step);action.addProperty("action",p.concede?"concede":"ok");
            if(!p.concede) {
                WorkerRegression.check(p.reply==null&&p.ok&&Set.of("Play","Keep").contains(b.okLabel),"Unexpected opening prompt; do not invent an answer");
                JsonObject premature=WorkerRegression.reply(b,p,"premature-concede");premature.addProperty("action","concede");WorkerRegression.reject(b,premature);
            }
            previous=p;b.accept(action);
            if(p.concede) {
                for(int i=0;i<1000&&!b.published.contains("\"finished\"");i++)Thread.sleep(10);
                WorkerRegression.check(b.game.isGameOver()&&b.human.conceded(),"Forge did not record concession");
                WorkerRegression.check(b.published.contains("\"result\":\"CPU\"")&&b.published.contains("\"conceded\":true"),"Missing real concession result");
                JsonObject stale=WorkerRegression.reply(b,p,"late-concede");stale.addProperty("action","concede");WorkerRegression.reject(b,stale);
                Files.writeString(out.resolve("checks.json"),"{\"passed\":true,\"actualControllerConcession\":true,\"result\":\"CPU\",\"prematureAndStaleRejected\":true}\n");return;
            }
        }
        throw new AssertionError("No stable priority boundary reached");
    }
}
