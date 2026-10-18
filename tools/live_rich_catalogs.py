#!/usr/bin/env python3
"""Isolated keyless smoke run; retains failures as well as successful source data."""
import argparse
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--server', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--python', type=Path, help='Optional interpreter with workers/ytmusic/requirements.txt installed')
    args=parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    repository=Path(__file__).resolve().parents[1]
    with tempfile.TemporaryDirectory(prefix='ommr-rich-') as directory:
        root=Path(directory)
        with socket.socket() as sock:
            sock.bind(('127.0.0.1',0))
            port=sock.getsockname()[1]
        configuration=f'port: {port}\ncredentials_dir: ""\ncache_dir: {root / "cache"}\nprovider_timeout: 15s\n'
        if args.python:
            configuration+='extractor:\n  enabled: true\n  python: '+json.dumps(str(args.python.resolve()))+'\n  script: '+json.dumps(str(repository/'workers/ytmusic/worker.py'))+'\n  workers: 2\n  timeout: 8s\n  profiles: true\n'
        config=root/'test.yaml'
        config.write_text(configuration)
        config.chmod(0o600)
        env={key:value for key,value in os.environ.items() if not key.startswith('OMMR_')}
        env['OMMR_ENV_FILE']=''
        base=f'http://127.0.0.1:{port}'
        cases=[(name,'extract',{'provider':name,'id':entity_id,'debug':'true'}) for name,entity_id in [('bugs','31999479'),('genie','90705547'),('melon','32872978')]]
        cases.append(('dynamite-korean','resolve',{'artist':'방탄소년단','title':'Dynamite','sources':'bugs,genie,melon','debug':'true'}))
        if args.python:
            cases.extend([
                ('youtube-song','catalog',{'provider':'ytmusic','type':'song','id':'tIjUfo30r4M','limit':5}),
                ('youtube-artist','catalog',{'provider':'ytmusic','type':'artist','id':'UCSmK5WX5U4gdtebWjoL81og','limit':5}),
                ('youtube-album','catalog',{'provider':'ytmusic','type':'album','id':'MPREb_qTDpBqltt6c','limit':5}),
                ('boyfriend-context','resolve',{'artist':'Karan Aujla','title':'Boyfriend','sources':'applemusic,ytmusic','debug':'true'}),
            ])
        with (args.output/'server.log').open('w') as log:
            process=subprocess.Popen([str(args.server.resolve()),'--config',str(config)],cwd=root,env=env,stdout=log,stderr=log)
            try:
                for _ in range(150):
                    if process.poll() is not None:
                        raise RuntimeError('server exited before becoming ready')
                    try:
                        with urllib.request.urlopen(base+'/v1/health',timeout=1):
                            break
                    except OSError:
                        time.sleep(.1)
                else:
                    raise RuntimeError('server did not become ready')
                rows=[]
                for name,operation,parameters in cases:
                    start=time.monotonic()
                    try:
                        with urllib.request.urlopen(base+'/v1/'+operation+'?'+urllib.parse.urlencode(parameters),timeout=20) as response:
                            data,status=json.load(response),response.status
                    except urllib.error.HTTPError as error:
                        data,status=json.loads(error.read()),error.code
                    except OSError:
                        data,status={'error':'request transport failed'},0
                    (args.output/(name+'.json')).write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n')
                    track=data.get('track') or next((item['track'] for source in data.get('sources',[]) for item in source.get('results',[])),{})
                    row={'case':name,'http_status':status,'seconds':round(time.monotonic()-start,3),'title':track.get('title'),'ids':track.get('ids'),'sources':[{key:source.get(key) for key in ('provider','status','error')} for source in data.get('sources',[])],'error':data.get('error')}
                    rows.append(row)
                    print(json.dumps(row,ensure_ascii=False),flush=True)
                (args.output/'summary.json').write_text(json.dumps(rows,ensure_ascii=False,indent=2)+'\n')
            finally:
                process.terminate()
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()


if __name__=='__main__':
    main()
