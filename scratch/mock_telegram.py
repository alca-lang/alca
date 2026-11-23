import json
import re
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

LOG = open('scratch/mock_requests.log', 'a', encoding='utf-8')


def log(s):
    print(s, flush=True)
    LOG.write(s + '\n')
    LOG.flush()


def parse_multipart(body, content_type):
    m = re.search(r'boundary=(.+)', content_type)
    if not m:
        return {}, {}
    boundary = m.group(1).strip('"').encode()
    parts = body.split(b'--' + boundary)
    fields = {}
    files = {}
    for part in parts:
        part = part.strip(b'\r\n')
        if not part or part == b'--':
            continue
        if b'\r\n\r\n' not in part:
            continue
        head, data = part.split(b'\r\n\r\n', 1)
        data = data.rstrip(b'\r\n')
        head_text = head.decode('utf-8', 'replace')
        nm = re.search(r'name="([^"]+)"', head_text)
        if not nm:
            continue
        name = nm.group(1)
        fn = re.search(r'filename="([^"]*)"', head_text)
        if fn:
            files[name] = {'filename': fn.group(1), 'size': len(data), 'head': head_text}
        else:
            fields[name] = data.decode('utf-8', 'replace')
    return fields, files


class H(BaseHTTPRequestHandler):
    def do_GET(self):
        log('GET ' + self.path)
        if self.path.startswith('/file/'):
            data = b'FILE-CONTENT-\x00\x01\x02'
            self.send_response(200)
            self.send_header('Content-Length', str(len(data)))
            self.end_headers()
            self.wfile.write(data)
            return
        self.reply({'ok': True, 'result': True})

    def do_POST(self):
        length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(length)
        ctype = self.headers.get('Content-Type', '')
        method = self.path.rsplit('/', 1)[-1]
        if 'multipart/form-data' in ctype:
            fields, files = parse_multipart(body, ctype)
            log('POST ' + method + ' MULTIPART fields=' + json.dumps(fields, ensure_ascii=False) + ' files=' + json.dumps(files, ensure_ascii=False))
        else:
            try:
                payload = json.loads(body.decode('utf-8'))
            except Exception:
                payload = body[:200].decode('utf-8', 'replace')
            log('POST ' + method + ' JSON ' + json.dumps(payload, ensure_ascii=False))
        if method == 'getMe':
            self.reply({'ok': True, 'result': {'id': 1, 'is_bot': True, 'first_name': 'Test', 'username': 'testbot'}})
        elif method == 'getUpdates':
            self.reply({'ok': True, 'result': [{
                'update_id': 100,
                'message': {
                    'message_id': 5,
                    'date': 1700000000,
                    'from': {'id': 7, 'is_bot': False, 'first_name': 'Alex', 'username': 'alex'},
                    'chat': {'id': 7, 'type': 'private', 'first_name': 'Alex', 'username': 'alex'},
                    'text': '/start hello world',
                },
            }]})
        elif method == 'getFile':
            self.reply({'ok': True, 'result': {'file_id': 'f1', 'file_unique_id': 'u1', 'file_path': 'documents/test.bin', 'file_size': 16}})
        elif method == 'sendPhoto':
            self.reply({'ok': True, 'result': {'message_id': 43, 'date': 1700000001, 'chat': {'id': 7, 'type': 'private'}, 'photo': [{'file_id': 'p', 'file_unique_id': 'pu', 'width': 1, 'height': 1}]}})
        elif method == 'sendMessage':
            self.reply({'ok': True, 'result': {'message_id': 42, 'date': 1700000001, 'chat': {'id': 7, 'type': 'private'}, 'text': 'x'}})
        else:
            self.reply({'ok': True, 'result': True})

    def reply(self, obj):
        data = json.dumps(obj).encode('utf-8')
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *args):
        pass


if __name__ == '__main__':
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8765
    log('mock server on ' + str(port))
    HTTPServer(('127.0.0.1', port), H).serve_forever()
