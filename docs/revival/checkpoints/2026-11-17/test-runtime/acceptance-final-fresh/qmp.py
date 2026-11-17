import json
import socket
import sys

with socket.create_connection(('127.0.0.1', 18768), timeout=5) as sock:
    stream = sock.makefile('rwb')
    print(json.loads(stream.readline()))

    def call(command, arguments=None):
        request = {'execute': command}
        if arguments is not None:
            request['arguments'] = arguments
        stream.write((json.dumps(request) + '\n').encode())
        stream.flush()
        while True:
            response = json.loads(stream.readline())
            if 'return' in response or 'error' in response:
                print(response)
                return response

    call('qmp_capabilities')
    identity = call('query-name')
    assert identity.get('return', {}).get('name') == 'nudanos-final-fresh'
    for command in sys.argv[1:]:
        call('human-monitor-command', {'command-line': command})
