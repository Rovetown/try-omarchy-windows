#!/usr/bin/env python3
"""Boot the packaged Windows binaries with real Unicode log and disk paths."""
import argparse
import json
from pathlib import Path
import subprocess
import tempfile

parser=argparse.ArgumentParser()
parser.add_argument('bin',type=Path)
args=parser.parse_args()
binary=args.bin.resolve()
with tempfile.TemporaryDirectory(prefix='tryomarchy-unicode-') as temporary:
    root=Path(temporary)/'Omarchy 世界 café'
    root.mkdir()
    disk=root/'guest 日本語.raw'
    created=subprocess.run([str(binary/'qemu-img.exe'),'create','-f','raw',str(disk),'64k'],
                           capture_output=True,timeout=15)
    assert created.returncode==0,created.stderr.decode('utf-8',errors='replace')
    info=json.loads(subprocess.check_output(
        [str(binary/'qemu-img.exe'),'info','--output=json',str(disk)],timeout=15))
    assert info['virtual-size']==65536
    for name in ('qemu-system-x86_64.exe','qemu-system-x86_64w.exe'):
        log=root/(name+'.log')
        trace=root/(name+'.serial')
        result=subprocess.run([str(binary/name),'-machine','none','-nodefaults',
            '-display','none','-S','-D',str(log),
            '-chardev',f'file,id=trace,path={trace}',
            '-drive',f'file={disk},format=raw,if=none,id=probe',
            '-qmp','stdio'],input=b'{"execute":"qmp_capabilities"}\n{"execute":"quit"}\n',
            stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=15)
        assert result.returncode==0,(name,result.stderr.decode('utf-8',errors='replace'))
        replies=[json.loads(line) for line in result.stdout.splitlines() if line.strip()]
        assert any('QMP' in r for r in replies), replies
        assert not any('error' in r for r in replies), replies
        assert log.is_file() and trace.is_file()
print('ok - both QEMU launchers and qemu-img accept Unicode paths')
