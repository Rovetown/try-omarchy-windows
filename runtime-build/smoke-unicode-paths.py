#!/usr/bin/env python3
"""Boot the packaged Windows binaries with real Unicode log and disk paths."""
import argparse
import json
from pathlib import Path
import runpy
import subprocess
import tempfile

parser=argparse.ArgumentParser()
parser.add_argument('bin',type=Path)
args=parser.parse_args()
binary=args.bin.resolve()
# Use the same bounded socket transport as the RAM smoke test. Windows QEMU
# does not reliably consume redirected stdio before the producer closes it.
VM = runpy.run_path(str(Path(__file__).with_name('smoke-memory.py')))['VM']
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
        with VM(binary/name, ['-D',str(log),
            '-chardev',f'file,id=trace,path={trace}',
            '-drive',f'file={disk},format=raw,if=none,id=probe']) as vm:
            assert vm.call('query-status')['status'] == 'prelaunch'
            vm.call('quit')
            assert vm.process.wait(timeout=10) == 0
        assert log.is_file() and trace.is_file()
print('ok - both QEMU launchers and qemu-img accept Unicode paths')
