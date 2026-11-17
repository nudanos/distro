"""Create a NoCloud ISO without host installation or external image tools."""
import io
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(ROOT / 'tools' / 'python-libs'))
import pycdlib

guest = ROOT / 'arch'
public_key = (guest / 'id_ed25519.pub').read_text().strip()
user_data = f'''#cloud-config
hostname: nudanos-arch-test
manage_etc_hosts: true
users:
  - name: builder
    groups: [wheel]
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL
    lock_passwd: true
    ssh_authorized_keys:
      - {public_key}
ssh_pwauth: false
disable_root: true
package_update: false
runcmd:
  - [systemctl, enable, --now, sshd.service]
growpart:
  mode: auto
  devices: ['/']
resize_rootfs: true
'''
metadata = 'instance-id: nudanos-arch-test-20261106b\nlocal-hostname: nudanos-arch-test\n'
iso = pycdlib.PyCdlib()
iso.new(interchange_level=3, joliet=3, rock_ridge='1.09', vol_ident='CIDATA')
for name, data in [('user-data', user_data), ('meta-data', metadata)]:
    raw = data.encode()
    (guest / name).write_bytes(raw)
    iso.add_fp(io.BytesIO(raw), len(raw), iso_path='/' + name.upper().replace('-', '_') + '.;1', rr_name=name, joliet_path='/' + name)
iso.write(str(guest / 'seed.iso'))
iso.close()
print('Created NoCloud seed ISO')


