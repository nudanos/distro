import crypt_r, os, subprocess
path = "/home/builder/revival-checks/vyatta-login/scripts/encrypt_shadow_passwd"
for password in ("x" * 511, "雪" * 170 + "x"):
    result = subprocess.run(["bash", path], env=dict(os.environ, SECRET=password), text=True, capture_output=True)
    assert result.returncode == 0, (result.returncode, result.stderr)
    assert crypt_r.crypt(password, result.stdout) == result.stdout
result = subprocess.run(["bash", path], env=dict(os.environ, SECRET="x" * 512), text=True, capture_output=True)
assert result.returncode != 0, (result.returncode, result.stdout, result.stderr)
print("libxcrypt input boundary verified")
