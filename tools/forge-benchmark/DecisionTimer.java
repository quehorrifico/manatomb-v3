// Measurement-only agent: brackets one existing AI method; leaves all rules/results intact.
import java.lang.instrument.*;
import java.security.ProtectionDomain;
import java.nio.file.*;
import jdk.internal.org.objectweb.asm.*;
public class DecisionTimer {
    private static final ThreadLocal<Long> START=new ThreadLocal<>();
    public static void begin(){START.set(System.nanoTime());append("begin,"+System.nanoTime()+","+Thread.currentThread().getId());}
    public static void end(){Long started=START.get();long finished=System.nanoTime();if(started!=null)append("end,"+finished+","+Thread.currentThread().getId()+","+(finished-started));START.remove();}
    private static synchronized void append(String line){try{Files.writeString(Path.of("/out/decisions.csv"),line+"\n",StandardOpenOption.CREATE,StandardOpenOption.APPEND);}catch(Exception ignored){}}
    public static void premain(String ignored,Instrumentation instrumentation){
        instrumentation.addTransformer(new ClassFileTransformer(){
            public byte[] transform(ClassLoader loader,String name,Class<?> redef,ProtectionDomain domain,byte[] data){
                if(!name.equals("forge/ai/PlayerControllerAi"))return null;
                ClassReader reader=new ClassReader(data);ClassWriter writer=new ClassWriter(reader,ClassWriter.COMPUTE_MAXS);
                reader.accept(new ClassVisitor(Opcodes.ASM8,writer){
                    public MethodVisitor visitMethod(int access,String method,String desc,String signature,String[] exceptions){
                        MethodVisitor delegate=super.visitMethod(access,method,desc,signature,exceptions);
                        if(!method.equals("chooseSpellAbilityToPlay")||!desc.equals("()Ljava/util/List;"))return delegate;
                        return new MethodVisitor(Opcodes.ASM8,delegate){
                            public void visitCode(){super.visitCode();super.visitMethodInsn(Opcodes.INVOKESTATIC,"DecisionTimer","begin","()V",false);}
                            public void visitInsn(int op){if(op==Opcodes.ARETURN||op==Opcodes.ATHROW)super.visitMethodInsn(Opcodes.INVOKESTATIC,"DecisionTimer","end","()V",false);super.visitInsn(op);}
                        };
                    }
                },0);append("instrumented,forge.ai.PlayerControllerAi.chooseSpellAbilityToPlay");return writer.toByteArray();
            }
        });
    }
}
