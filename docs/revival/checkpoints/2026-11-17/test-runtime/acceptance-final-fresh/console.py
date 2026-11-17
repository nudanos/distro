"""Paced passive serial relay for the final-candidate disposable appliance.

This never supplies a kernel/init override or changes the guest. Input remains
available during boot and installer prompts. EOF on controller stdin does not
terminate QEMU; commands.queue provides an independent complete-line channel.
"""
import argparse
import hashlib
import json
import subprocess
import sys
import threading
import time
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
QEMU = ROOT / 'test-runtime/tools/qemu/qemu-system-x86_64.exe'


def paced_send(stream, value, lock, delay=.04):
    with lock:
        for byte in (value + '\n').encode('utf-8'):
            stream.write(bytes([byte]))
            stream.flush()
            time.sleep(delay)


def complete_lines(pending, fragment):
    pieces = (pending + fragment).split('\n')
    return pieces[-1], [line.rstrip('\r') for line in pieces[:-1]]


def verified_iso(manifest):
    if manifest.get('status') != 'final':
        raise ValueError('Builder has not designated this ISO as final')
    iso = Path(manifest['iso_path']).resolve(strict=True)
    with iso.open('rb') as source:
        actual = hashlib.file_digest(source, 'sha256').hexdigest()
    if actual != manifest['iso_sha256'].lower():
        raise ValueError('Final ISO hash mismatch')
    return iso


def main():
    sys.stdout.reconfigure(encoding='utf-8', errors='backslashreplace')
    sys.stderr.reconfigure(encoding='utf-8', errors='backslashreplace')
    parser = argparse.ArgumentParser()
    parser.add_argument('mode', choices=['live', 'installed'])
    parser.add_argument('--log', required=True)
    args = parser.parse_args()
    manifest = json.loads((HERE / 'final-artifact.json').read_text())
    iso = verified_iso(manifest)
    disk = (HERE / 'fresh.qcow2').resolve(strict=True)
    logfile = (HERE / args.log).resolve()
    if logfile.parent != HERE or logfile.exists():
        raise ValueError('Use a new transcript basename in this fixture')
    command = [str(QEMU), '-name', 'nudanos-final-fresh', '-machine', 'q35',
               '-accel', 'whpx', '-cpu', 'qemu64,-svm', '-smp', '2', '-m', '2048',
               '-display', 'none', '-monitor', 'none',
               '-qmp', 'tcp:127.0.0.1:18768,server=on,wait=off',
               '-serial', 'stdio', '-drive', f'file={disk},if=virtio,format=qcow2',
               '-netdev', 'user,id=m,hostfwd=tcp:127.0.0.1:2226-:22',
               '-device', 'virtio-net-pci,netdev=m,addr=10,mac=52:54:00:00:01:16']
    if args.mode == 'live':
        command += ['-cdrom', str(iso), '-boot', 'd']
    queue = HERE / 'commands.queue'
    queue.touch(exist_ok=True)
    log = logfile.open('xb', buffering=0)
    process = subprocess.Popen(command, stdin=subprocess.PIPE,
                               stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                               bufsize=0)
    (HERE / 'active-process.json').write_text(json.dumps({
        'pid': process.pid, 'command': command, 'log': str(logfile),
        'iso_sha256': manifest['iso_sha256']}, indent=2))
    sendlock = threading.Lock()

    def send(value):
        try:
            paced_send(process.stdin, value, sendlock)
        except (BrokenPipeError, OSError):
            return

    def reader():
        while chunk := process.stdout.read(1):
            log.write(chunk)

    def stdin_forward():
        for line in sys.stdin:
            send(line.rstrip('\r\n'))

    def queue_forward():
        with queue.open('r', encoding='utf-8') as source:
            source.seek(0, 2)
            pending = ''
            while process.poll() is None:
                fragment = source.read()
                if fragment:
                    pending, lines = complete_lines(pending, fragment)
                    for line in lines:
                        send(line)
                else:
                    time.sleep(.2)

    output = threading.Thread(target=reader)
    output.start()
    threading.Thread(target=stdin_forward, daemon=True).start()
    threading.Thread(target=queue_forward, daemon=True).start()
    print(f'QEMU PID {process.pid}; passive stock {args.mode} boot; log {logfile}',
          flush=True)
    result = process.wait()
    output.join()
    log.close()
    print(f'QEMU exited {result}', flush=True)
    return result


if __name__ == '__main__':
    raise SystemExit(main())
