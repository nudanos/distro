import re, subprocess, sys, threading, time
from pathlib import Path
sys.stdout.reconfigure(encoding='utf-8', errors='backslashreplace')
root = Path(__file__).resolve().parents[2]
qemu = root/'test-runtime/tools/qemu/qemu-system-x86_64.exe'
disk = root/'test-runtime/acceptance-interim/interim.qcow2'
p = subprocess.Popen([str(qemu), '-name','nudanos-baseline-installed', '-machine','q35', '-accel','whpx', '-cpu','qemu64,-svm', '-smp','2', '-m','2048', '-display','none', '-monitor','none', '-serial','stdio', '-drive',f'file={disk},if=virtio,format=qcow2', '-qmp','tcp:127.0.0.1:18770,server=on,wait=off', '-netdev','user,id=m,hostfwd=tcp:127.0.0.1:2228-:22', '-device','virtio-net-pci,netdev=m,addr=10,mac=52:54:00:00:01:10'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, bufsize=0)
log = open(root/'test-runtime/acceptance-interim/interim-boot.log','wb',buffering=0)
buf, lock, sendlock = [''], threading.Lock(), threading.Lock()
def reader():
    while b := p.stdout.read(1):
        log.write(b)
        with lock: buf[0] += b.decode('latin1')
threading.Thread(target=reader,daemon=True).start()
def send(s):
    with sendlock:
        for b in (s+'\n').encode(): p.stdin.write(bytes([b])); p.stdin.flush(); time.sleep(.01)
def forward():
    for line in sys.stdin: send(line.rstrip('\r\n'))
threading.Thread(target=forward,daemon=True).start()
def expect(patterns, timeout):
    elapsed=0.; prev=time.monotonic()
    while elapsed < timeout:
        now=time.monotonic(); gap=now-prev; prev=now
        if gap<5: elapsed += gap
        with lock:
            data=re.sub(r'\x1b\[[0-9;?]*[a-zA-Z]', '', buf[0])
            for i,pattern in enumerate(patterns):
                m=re.search(pattern,data,re.S)
                if m: buf[0]=data[m.end():]; return i
        if p.poll() is not None: raise RuntimeError('QEMU exited early')
        time.sleep(.2)
    raise RuntimeError('Active timeout: '+buf[0][-1000:])
try:
    expect([r'login: $'],1800)
    for _ in range(30):
        send('nudanos')
        i=expect([r'Password: $',r'login: $'],300)
        if i: continue
        send('<disposable-fixture-password>')
        i=expect([r':~\$ $',r'Login incorrect',r'login: $'],300)
        if i==0: break
        if i==1: expect([r'login: $'],120)
        time.sleep(5)
    else: raise RuntimeError('Installed admin login failed')
    print('Installed baseline admin login succeeded',flush=True)
    while p.poll() is None: time.sleep(.2)
except BaseException as e:
    print('FAILED:',str(e).encode('ascii','backslashreplace').decode('ascii'),flush=True)
    if p.poll() is None: p.terminate(); p.wait(timeout=30)
    sys.exit(1)
finally: log.close()

