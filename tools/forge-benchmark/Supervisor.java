// Local measurement sidecar in the same cgroup. It never chooses gameplay answers.
import com.google.gson.*;
import com.sun.net.httpserver.*;
import java.net.*;
import java.net.http.*;
import java.nio.file.*;
import java.io.*;
import java.util.*;
import java.util.concurrent.*;

public class Supervisor {
    static final Gson JSON=new Gson();
    static final Path OUT=Path.of("/out");
    static long start=System.nanoTime();
    static Process child;
    static volatile boolean sampling=true;
    static final HttpClient CLIENT=HttpClient.newBuilder().connectTimeout(java.time.Duration.ofSeconds(1)).build();
    static final Object logLock=new Object();
    static double seconds(){return (System.nanoTime()-start)/1e9;}
    static void event(String type,Object value){synchronized(logLock){try{Files.writeString(OUT.resolve("events.jsonl"),JSON.toJson(Map.of("seconds",seconds(),"event",type,"value",value))+"\n",StandardOpenOption.CREATE,StandardOpenOption.APPEND);}catch(IOException e){throw new UncheckedIOException(e);}}}
    static String read(String path){try{return Files.readString(Path.of(path)).trim();}catch(IOException e){return "";}}
    static long field(String text,String key){for(String line:text.split("\n")){String[] bits=line.trim().split("\\s+");if(bits.length>1&&bits[0].equals(key))return Long.parseLong(bits[1]);}return -1;}
    static HttpResponse<byte[]> engine(String method,String path,byte[] body)throws Exception {
        var b=HttpRequest.newBuilder(URI.create("http://127.0.0.1:18711"+path)).timeout(java.time.Duration.ofSeconds(3));
        if(method.equals("POST"))b.header("Origin","http://127.0.0.1:18711").header("Content-Type","application/json").POST(HttpRequest.BodyPublishers.ofByteArray(body));else b.GET();
        return CLIENT.send(b.build(),HttpResponse.BodyHandlers.ofByteArray());
    }
    static void cancel(boolean force)throws Exception{
        event(force?"force-kill":"cancel",Map.of("alive",child.isAlive()));
        if(force)child.destroyForcibly();else child.destroy();
        if(!child.waitFor(3,TimeUnit.SECONDS)){event("cancel-escalated",true);child.destroyForcibly();child.waitFor(3,TimeUnit.SECONDS);}
        event("worker-reaped",Map.of("alive",child.isAlive(),"exit",child.isAlive()?-999:child.exitValue()));
    }
    public static void main(String[] args)throws Exception {
        Files.createDirectories(OUT);
        var argv=Files.readAllLines(Path.of("/config/engine.argv"));
        event("environment",Map.of("arch",System.getProperty("os.arch"),"os",System.getProperty("os.name"),"java",System.getProperty("java.runtime.version"),"memory_max",read("/sys/fs/cgroup/memory.max"),"swap_max",read("/sys/fs/cgroup/memory.swap.max"),"cpu_max",read("/sys/fs/cgroup/cpu.max")));
        child=new ProcessBuilder(argv).directory(Path.of("/forge/forge-26d8aff87509dde8a9d5017852339382d7ab3e83/forge-gui-desktop").toFile()).redirectErrorStream(true).redirectOutput(OUT.resolve("engine.log").toFile()).start();
        event("worker-start",child.pid());
        Runtime.getRuntime().addShutdownHook(new Thread(()->{try{cancel(false);}catch(Exception ignored){}sampling=false;}));
        new Thread(()->{
            String previous="";boolean ready=false;int ticks=0;
            try(var writer=Files.newBufferedWriter(OUT.resolve("samples.csv"))){
                writer.write("seconds,memory_current,memory_peak,anon,file,kernel,cpu_usec,worker_rss_kib,worker_threads,worker_alive\n");
                while(sampling){
                    String stat=read("/sys/fs/cgroup/memory.stat"),proc=read("/proc/"+child.pid()+"/status");
                    writer.write(String.format(Locale.ROOT,"%.6f,%s,%s,%d,%d,%d,%d,%d,%d,%b%n",seconds(),read("/sys/fs/cgroup/memory.current"),read("/sys/fs/cgroup/memory.peak"),field(stat,"anon"),field(stat,"file"),field(stat,"kernel"),field(read("/sys/fs/cgroup/cpu.stat"),"usage_usec"),field(proc,"VmRSS:"),field(proc,"Threads:"),child.isAlive()));writer.flush();
                    if(ticks++%4==0 && child.isAlive())try{
                        var response=engine("GET","/state",null);String raw=new String(response.body(),java.nio.charset.StandardCharsets.UTF_8);JsonObject dto=JsonParser.parseString(raw).getAsJsonObject();
                        String status=dto.get("status").getAsString();
                        if(!ready && Set.of("input","choice").contains(status)){ready=true;event("ready",status);}
                        String signature=status+":"+dto.get("revision");
                        if(!signature.equals(previous)){previous=signature;Map<String,Object> safe=new LinkedHashMap<>();for(String k:List.of("status","revision","turn","turnPlayer","phase","result"))if(dto.has(k))safe.put(k,dto.get(k));if(dto.has("prompt"))safe.put("promptKind",dto.getAsJsonObject("prompt").get("kind"));event("view",safe);}
                    }catch(Exception ignored){}
                    if(!child.isAlive())Files.writeString(OUT.resolve("worker-exit.json"),JSON.toJson(Map.of("exit",child.exitValue(),"memory_events",read("/sys/fs/cgroup/memory.events"))));
                    Thread.sleep(250);
                }
            }catch(Exception e){event("sampler-error",e.toString());}
        },"cgroup-sampler").start();
        HttpServer server=HttpServer.create(new InetSocketAddress("0.0.0.0",18712),0);server.setExecutor(Executors.newFixedThreadPool(4));
        server.createContext("/",exchange->{
            long began=System.nanoTime();String path=exchange.getRequestURI().getPath(),method=exchange.getRequestMethod();int code=500;byte[] response="{}".getBytes();
            try{
                if(!Objects.equals(exchange.getRequestHeaders().getFirst("Host"),"127.0.0.1:18711"))throw new IllegalArgumentException("host");
                if(method.equals("POST")&&!Objects.equals(exchange.getRequestHeaders().getFirst("Origin"),"http://127.0.0.1:18711"))throw new IllegalArgumentException("origin");
                byte[] body=exchange.getRequestBody().readNBytes(16385);if(body.length>16384)throw new IllegalArgumentException("size");
                if(path.equals("/bench/cancel")||path.equals("/bench/kill")){
                    if(!method.equals("POST"))throw new IllegalArgumentException("method");cancel(path.endsWith("kill"));code=200;
                }else if(Set.of("/","/state","/action").contains(path)){
                    var r=engine(method,path,body);code=r.statusCode();response=r.body();exchange.getResponseHeaders().set("Content-Type",r.headers().firstValue("Content-Type").orElse("application/json"));
                }else code=404;
                event("http",Map.of("method",method,"path",path,"code",code,"request_bytes",body.length,"response_bytes",response.length,"milliseconds",(System.nanoTime()-began)/1e6));
            }catch(Exception e){event("proxy-error",e.getClass().getSimpleName());code=502;}
            try{exchange.getResponseHeaders().set("Cache-Control","no-store");exchange.sendResponseHeaders(code,response.length);exchange.getResponseBody().write(response);}catch(IOException e){event("interrupted-response",path);}finally{exchange.close();}
        });server.start();
    }
}
