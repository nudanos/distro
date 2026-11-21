import os, pathlib, subprocess, tempfile, time, itertools
root = pathlib.Path('/home/builder/startup-lock-tests')
def wait_for(path, process):
    deadline=time.monotonic()+8
    while time.monotonic()<deadline:
        if path.exists(): return
        if process.poll() is not None:
            out,err=process.communicate()
            raise RuntimeError(f'process exited {process.returncode}: {out!r} {err!r}')
        time.sleep(.02)
    raise RuntimeError(f'no marker {path}')
with tempfile.TemporaryDirectory(prefix='cross-',dir=root) as tmp:
    base=pathlib.Path(tmp); config=base/'etc/config'; config.mkdir(parents=True)
    sbin=base/'sbin'; sbin.mkdir(); bindir=base/'bin'; bindir.mkdir()
    data='system {\n login {\n  user tester {\n   authentication {\n    encrypted-password "$6$old$hash"\n   }\n  }\n }\n}\n'
    version='// === vyatta-config-version: "system@1" ===\n'
    (base/'data').write_text(data); (config/'config.boot').write_text(data+version); (config/'config.boot').chmod(0o600)
    (sbin/'vyatta_current_conf_ver.pl').write_text('#!/bin/sh\nprintf \'%s\\n\' \'// === vyatta-config-version: "system@1" ===\'\n'); (sbin/'vyatta_current_conf_ver.pl').chmod(0o755)
    cli=bindir/'cli-shell-api'
    cli.write_text('#!/bin/sh\ntouch "$BASE/$ID.entered"\nwhile [ ! -e "$BASE/$ID.release" ]; do sleep .02; done\ncat "$BASE/data"\n'); cli.chmod(0o755)
    script=(root/'cfg/scripts/vyatta-save-config.pl').read_text().replace('"/opt/vyatta/etc"',repr(str(base/'etc'))).replace('"/opt/vyatta/sbin"',repr(str(sbin))).replace('/run/nudanos-startup',str(base/'lock'))
    save=base/'save.pl'; save.write_text(script)
    def launch(kind,ident):
        env=os.environ.copy(); env.update(BASE=str(base),ID=ident,PATH=str(bindir)+':'+env['PATH'])
        if kind=='python': cmd=['python3',str(root/'cross-lock-python.py'),str(base),ident]
        elif kind=='go': cmd=[str(root/'cross-lock-go'),str(base),ident]
        else: cmd=['perl','-I'+str(root/'cfg/lib'),'-I'+str(root/'cfg/test/lib'),str(save)]
        return subprocess.Popen(cmd,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
    for n,(first,second) in enumerate(itertools.permutations(('python','go','perl'),2)):
        a,b=f'case{n}a',f'case{n}b'; p=launch(first,a); q=None
        try:
            wait_for(base/(a+'.entered'),p)
            inode=(base/'lock/startup.lock').stat().st_ino
            q=launch(second,b); time.sleep(.5)
            if q.poll() is not None or (base/(b+'.entered')).exists():
                raise AssertionError(f'{second} did not wait for {first}')
            (base/(a+'.release')).touch(); wait_for(base/(b+'.entered'),q)
            (base/(b+'.release')).touch()
            for x in (p,q):
                out,err=x.communicate(timeout=5)
                if x.returncode: raise AssertionError(f'{x.args}: {out} {err}')
            assert (base/'lock/startup.lock').stat().st_ino==inode
            print(f'{first} held, {second} waited before entering: PASS',flush=True)
        finally:
            (base/(a+'.release')).touch(); (base/(b+'.release')).touch()
            for x in (p,q):
                if x and x.poll() is None: x.kill(); x.wait()
