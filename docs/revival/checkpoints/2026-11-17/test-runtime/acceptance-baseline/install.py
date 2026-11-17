import os, re, subprocess, sys, threading, time
from pathlib import Path
sys.stdout.reconfigure(encoding='utf-8', errors='backslashreplace')
sys.stderr.reconfigure(encoding='utf-8', errors='backslashreplace')

root = Path(__file__).resolve().parents[2]
qemu = root / 'test-runtime/tools/qemu/qemu-system-x86_64.exe'
disk = root / 'test-runtime/acceptance-baseline/baseline-install.qcow2'
iso = root / 'test-runtime/baseline-image/nudanos-1.0~20261023-amd64.iso'
args = [str(qemu), '-name', 'nudanos-baseline-acceptance', '-machine', 'pc', '-accel', 'tcg,thread=multi', '-cpu', 'max', '-smp', '2', '-m', '2048', '-display', 'none', '-monitor', 'none', '-serial', 'stdio', '-drive', f'file={disk},if=virtio,format=qcow2', '-netdev', 'user,id=m', '-device', 'virtio-net-pci,netdev=m,addr=10,mac=52:54:00:00:01:10', '-cdrom', str(iso), '-boot', 'd']
print('Starting baseline QEMU; transcript is baseline-install.log', flush=True)
p = subprocess.Popen(args, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, bufsize=0)
state = {'buf': '', 'last': time.monotonic()}
lock = threading.Lock()
log = open(root / 'test-runtime/acceptance-baseline/baseline-install.log', 'wb', buffering=0)
def reader():
    while True:
        b = p.stdout.read(1)
        if not b: break
        log.write(b)
        with lock:
            state['buf'] += b.decode('latin1')
            state['last'] = time.monotonic()
threading.Thread(target=reader, daemon=True).start()
def expect(patterns, timeout):
    elapsed, previous = 0.0, time.monotonic()
    while elapsed < timeout:
        now = time.monotonic()
        gap = now - previous
        if gap < 5: elapsed += gap
        previous = now
        with lock:
            text = re.sub(r'\x1b\[[0-9;?]*[a-zA-Z]', '', state['buf'])
            for i, pattern in enumerate(patterns):
                m = re.search(pattern, text, re.S)
                if m:
                    state['buf'] = text[m.end():]
                    return i, m.group(0)
        if p.poll() is not None: raise RuntimeError(f'QEMU exited {p.returncode}; tail {text[-1000:]}')
        time.sleep(.2)
    raise RuntimeError(f'timed out; tail {state["buf"][-1500:]}')
def send(value):
    for b in (value + '\n').encode():
        p.stdin.write(bytes([b])); p.stdin.flush(); time.sleep(.01)
def wait_exit(timeout):
    elapsed, prev = 0., time.monotonic()
    while p.poll() is None:
        now = time.monotonic(); gap = now - prev; prev = now
        if gap < 5: elapsed += gap
        if elapsed >= timeout: raise RuntimeError('QEMU shutdown active-time timeout')
        time.sleep(.2)
try:
    expect([r'login: $'], 1800)
    for attempt in range(30):
        send('vyatta')
        i, _ = expect([r'Password: $', r'login: $'], 300)
        if i == 1:
            time.sleep(10); continue
        send('vyatta')
        i, _ = expect([r':~\$ $', r'login: $'], 300)
        if i == 0: break
        time.sleep(10)
    else: raise RuntimeError('Live login failed repeatedly')
    print('Live login succeeded', flush=True)
    send('install image')
    patterns = [r'Done\.\r*\n', r'Continue[^\r\n]*\(Yes/No\) \[No\]:? ?$', r'Enter username for administrator account: $', r"(?:Enter|Retype) password for user '[^']+':$", r'(?:\?|\)|console|speed) \[[^\]\r\n]*\]:? ?$']
    seen = {}
    while True:
        i, m = expect(patterns, 1800)
        seen[m] = seen.get(m, 0) + 1
        print('Installer:', repr(m), flush=True)
        if seen[m] > 4: raise RuntimeError('Repeated installer prompt')
        if i == 0: break
        send(['', 'Yes', 'nudanos', '<disposable-fixture-password>', ''][i])
    expect([r':~\$ $'], 300); send('poweroff')
    expect([r'Proceed with poweroff\? \(Yes/No\) \[No\] ?$'], 120); send('y')
    wait_exit(600)
    print('Baseline installation and clean shutdown complete:', p.returncode, flush=True)
except BaseException as e:
    print('FAILED:', str(e).encode('ascii', 'backslashreplace').decode('ascii'), flush=True)
    if p.poll() is None:
        p.terminate(); p.wait(timeout=30)
    sys.exit(1)
finally: log.close()
