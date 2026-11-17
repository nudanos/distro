import io
import tempfile
import threading
from pathlib import Path
from unittest.mock import patch
import console

pending, lines = console.complete_lines('', 'partial')
assert pending == 'partial' and lines == []
pending, lines = console.complete_lines(pending, ' command\r\nnext\nlast')
assert pending == 'last' and lines == ['partial command', 'next']
output = io.BytesIO()
with patch('console.time.sleep') as sleep:
    console.paced_send(output, 'abc', threading.Lock())
    assert output.getvalue() == b'abc\n'
    assert sleep.call_count == 4
    assert all(call.args == (.04,) for call in sleep.call_args_list)
try:
    console.verified_iso({'status': 'provisional'})
except ValueError:
    pass
else:
    raise AssertionError('Provisional ISO accepted')
with tempfile.TemporaryDirectory(dir=console.HERE) as directory:
    iso = Path(directory) / 'fixture.iso'
    iso.write_bytes(b'fixture')
    try:
        console.verified_iso({'status': 'final', 'iso_path': str(iso),
                              'iso_sha256': '0' * 64})
    except ValueError:
        pass
    else:
        raise AssertionError('Wrong ISO hash accepted')
print('Pacing, partial queue lines and final ISO guard checks passed')
