import re, subprocess, sys, threading, time
from pathlib import Path
sys.stdout.reconfigure(encoding='utf-8', errors='backslashreplace')
root = Path(__file__).resolve().parents[2]
qemu = root / 'test-runtime/tools/qemu/qemu-system-x86_64.exe'
disk = root / 'test-runtime/acceptance-baseline/baseline-install.qcow2'
iso = root / 'test-runtime/baseline-image/nudanos-1.0~20261023-amd64.iso'
boot = root / 'test-runtime/acceptance-baseline/boot/live'
cmdline = 'boot=live components console=tty0 console=ttyS0,115200 net.ifnames=1 nopersistent nonetworking noeject systemd.show_status=1 init=/bin/bash'
args = [str(qemu), '-name', 'nudanos-baseline-rescue', '-machine', 'q35', '-accel', 'whpx', '-cpu', 'qemu64,-svm', '-smp', '2', '-m', '2048', '-display', 'none', '-monitor', 'none', '-serial', 'stdio', '-drive', f'file={disk},if=virtio,format=qcow2', '-netdev', 'user,id=m', '-device', 'virtio-net-pci,netdev=m,addr=10,mac=52:54:00:00:01:10', '-cdrom', str(iso), '-kernel', str(boot/'vmlinuz'), '-initrd', str(boot/'initrd.img'), '-append', cmdline]
p = subprocess.Popen(args, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, bufsize=0)
buf = ['']
lock = threading.Lock()
send_lock = threading.Lock()
log = open(root/'test-runtime/acceptance-baseline/rescue.log', 'wb', buffering=0)
def reader():
    while b := p.stdout.read(1):
        log.write(b)
        with lock: buf[0] += b.decode('latin1')
threading.Thread(target=reader, daemon=True).start()
def send(s):
    # A QEMU UART can drop long bursts before the guest drains its FIFO.
    with send_lock:
        for b in (s+'\n').encode():
            p.stdin.write(bytes([b])); p.stdin.flush(); time.sleep(.01)
queue = root/'test-runtime/acceptance-baseline/commands.queue'
queue.touch(exist_ok=True)
def stdin_forward():
    for line in sys.stdin: send(line.rstrip('\r\n'))
def queue_forward():
    with queue.open('r', encoding='utf-8') as f:
        f.seek(0, 2)
        pending = ''
        while p.poll() is None:
            line = f.readline()
            if line:
                pending += line
                if pending.endswith('\n'):
                    send(pending.rstrip('\r\n')); pending = ''
            else: time.sleep(.2)
threading.Thread(target=stdin_forward, daemon=True).start()
threading.Thread(target=queue_forward, daemon=True).start()
def expect(patterns, timeout):
    elapsed = 0.; prev = time.monotonic()
    while elapsed < timeout:
        now = time.monotonic(); gap = now-prev; prev=now
        if gap < 5: elapsed += gap
        with lock:
            data = re.sub(r'\x1b\[[0-9;?]*[a-zA-Z]', '', buf[0])
            for i, pattern in enumerate(patterns):
                m = re.search(pattern, data, re.S)
                if m:
                    buf[0] = data[m.end():]; return i
        if p.poll() is not None: raise RuntimeError('QEMU exited early')
        time.sleep(.2)
    raise RuntimeError('Active timeout; serial tail: '+buf[0][-1200:])
try:
    expect([r'(?:bash-[0-9.]+|root@[^\r\n]*)[#\$] $'], 1800)
    print('Root init shell reached; setting temporary live-overlay root hash', flush=True)
    send('<disposable-rescue-command-redacted>')
    expect([r'\r?\nRESCUE_HASH_SET\r?\n'], 120)
    expect([r'login: $'], 1800)
    for attempt in range(15):
        send('root')
        i = expect([r'Password: $', r'login: $'], 300)
        if i == 1: continue
        send('<disposable-fixture-password>')
        i = expect([r'root@[^\r\n]*# $', r'Login incorrect', r'login: $'], 300)
        if i != 0:
            if i == 1: expect([r'login: $'], 120)
            continue
        print('Rescue root login succeeded; polling boot configuration state', flush=True)
        send("r=TIMEOUT; for i in $(seq 1 180); do s=$(systemctl show -p ActiveState --value system-configure); case $s in active|failed) r=$s; break;; esac; sleep 2; done; echo CONFIG_STATE_$r")
        i = expect([r'\r?\nCONFIG_STATE_(?:active|inactive|failed|TIMEOUT)\r?\n', r'login: $'], 600)
        if i == 1: continue
        expect([r'root@[^\r\n]*# $'], 120)
        print('Capturing runtime diagnostics', flush=True)
        send("id vyatta; chage -l vyatta; getent shadow vyatta | cut -d: -f1,3-9; command -v mkpasswd || true; dpkg-query -W whois 2>&1; systemctl show -p ActiveState -p SubState -p Result system-configure; systemctl is-active configd; journalctl -b -u system-configure --no-pager | tail -100; echo INSPECTION_COMPLETE")
        i = expect([r'\r?\nINSPECTION_COMPLETE\r?\n', r'login: $'], 600)
        if i == 1: continue
        i = expect([r'root@[^\r\n]*# $', r'login: $'], 120)
        if i == 0: break
    else: raise RuntimeError('Rescue login/inspection repeatedly interrupted')
    print('Inspection captured; VM remains available for next steps', flush=True)
    # Keep the serial connection alive for subsequent commands from the session.
    while p.poll() is None: time.sleep(.2)
except BaseException as e:
    print('FAILED:', str(e).encode('ascii','backslashreplace').decode('ascii'), flush=True)
    if p.poll() is None: p.terminate(); p.wait(timeout=30)
    sys.exit(1)
finally: log.close()
