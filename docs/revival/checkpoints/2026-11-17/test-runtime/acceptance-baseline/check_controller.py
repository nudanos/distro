"""Replay fragmented console output through the saved expect implementations."""
import ast, re, threading, time
from pathlib import Path

class Running:
    def poll(self): return None

base = Path(__file__).parent
for name in ('install.py', 'seed-login.py', 'rescue.py'):
    tree = ast.parse((base/name).read_text())
    fn = next(x for x in tree.body if isinstance(x, ast.FunctionDef) and x.name == 'expect')
    scope = {'re': re, 'time': time, 'p': Running(), 'lock': threading.Lock()}
    if name == 'rescue.py': scope['buf'] = ['']
    else: scope['state'] = {'buf': '', 'last': time.monotonic()}
    exec(compile(ast.Module(body=[fn], type_ignores=[]), str(base/name), 'exec'), scope)
    def append(s):
        with scope['lock']:
            if name == 'rescue.py': scope['buf'][0] += s
            else: scope['state']['buf'] += s
    def producer():
        for part in ('noise\r\nnode log', 'in: ', 'Password', ': '):
            append(part); time.sleep(.03)
    t = threading.Thread(target=producer); t.start()
    first = scope['expect']([r'login: '], 3)
    second = scope['expect']([r'Password: '], 3)
    t.join()
    assert (first == 0 if name == 'rescue.py' else first[0] == 0)
    assert (second == 0 if name == 'rescue.py' else second[0] == 0)
    print(name, 'fragmented prompt replay passed')
marker = re.compile(r'\r?\nINSPECTION_COMPLETE\r?\n')
assert not marker.search('echo INSPECTION_COMPLETE\r\n')
assert marker.search('echo INSPECTION_COMPLETE\r\nINSPECTION_COMPLETE\r\n')
print('standalone marker replay passed')
