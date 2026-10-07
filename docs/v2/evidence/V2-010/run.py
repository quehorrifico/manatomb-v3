#!/usr/bin/env python3
"""Reproduce V2-010 in its dedicated external build directory; no browser bridge."""
import argparse
import json
from pathlib import Path
import shlex
import shutil
import subprocess
import time
import xml.etree.ElementTree as ET

PIN = "26d8aff87509dde8a9d5017852339382d7ab3e83"
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("work", type=Path, help="Dedicated directory containing the extracted source and Maven cache")
parser.add_argument("mode", choices=("sim", "probe"))
parser.add_argument("--java", default=shutil.which("java"))
parser.add_argument("--dry-run", action="store_true", help="Print argv/cwd without writing configuration or running Java")
args = parser.parse_args()
work = args.work.resolve()
source = work / f"forge-{PIN}"
runtime = work / "runtime"
desktop = source / "forge-gui-desktop"
namespace = {"m": "http://maven.apache.org/POM/4.0.0"}
pom = ET.parse(desktop / "pom.xml")
opens = shlex.split(pom.find("m:properties/m:addopen.java.args", namespace).text)
classpath = str(desktop / "target/classes") + ":" + (desktop / "target/runtime-classpath.txt").read_text().strip()
if not args.java or not all(Path(p).exists() for p in classpath.split(":")):
    parser.error("Java or built runtime classpath is missing; run the documented Maven command first")
command = [args.java, "-Djava.awt.headless=true", "-Dsentry.enabled=false", "-Xmx2g", *opens, "-cp", classpath]
if args.mode == "sim":
    command += ["forge.view.Main", "sim", "-d", "feline.dck", "hostility.dck", "-n", "1",
                "-f", "Commander", "-s", "20260912", "-a", "Default", "Default", "-c", "120"]
else:
    command += [str(Path(__file__).with_name("ResourceProbe.java").resolve()), "feline.dck", "hostility.dck"]
invocation = {"cwd": str(desktop), "argv": command}
if args.dry_run:
    print(json.dumps(invocation, indent=2))
    raise SystemExit(0)

# These paths are exclusively for this probe. Do not use an existing player profile.
for directory in ("user/preferences", "cache", "decks/commander"):
    (runtime / directory).mkdir(parents=True, exist_ok=True)
(source / "forge-gui/forge.profile.properties").write_text("".join(
    f"{key}={runtime / value}\n" for key, value in
    (("userDir", "user"), ("cacheDir", "cache"), ("decksDir", "decks"))))
(runtime / "user/preferences/forge.preferences").write_text(
    "LOAD_CARD_SCRIPTS_LAZILY=true\nDECKGEN_CARDBASED=false\nMULLIGAN_RULE=London\n"
    "UI_ENABLE_SOUNDS=false\nUI_ENABLE_MUSIC=false\n")
for name, filename in (("Feline Ferocity", "feline.dck"), ("Open Hostility", "hostility.dck")):
    shutil.copyfile(source / f"forge-gui/res/quest/precons/{name}.dck", runtime / "decks/commander" / filename)

(work / f"{args.mode}-reproduction-command.json").write_text(json.dumps(invocation, indent=2) + "\n")
log_path = work / f"{args.mode}-reproduction.log"
started = time.monotonic()
with log_path.open("w") as log:
    result = subprocess.run(command, cwd=desktop, stdout=log, stderr=subprocess.STDOUT, timeout=240)
print(json.dumps({"exit_code": result.returncode, "elapsed_seconds": round(time.monotonic() - started, 3), "log": str(log_path)}))
raise SystemExit(result.returncode)
