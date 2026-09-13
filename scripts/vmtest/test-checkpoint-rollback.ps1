param(
 [Parameter(Mandatory=$true)][string]$Launcher,
 [string]$WorkRoot=(Join-Path $env:TEMP ('TryOmarchyRollbackTest-'+[Guid]::NewGuid().ToString('N'))),
 [string]$ResultPath=(Join-Path $env:TEMP 'tryomarchy-rollback-result.json')
)
# Run in an interactive Windows session. Uses a new synthetic installation.
$ErrorActionPreference='Stop' 
$dir=$WorkRoot
Add-Type @'
using System;
using System.Runtime.InteropServices;
public class SnapshotUI {
 public delegate bool EnumProc(IntPtr h,IntPtr l);
 [DllImport("user32.dll")] public static extern bool EnumWindows(EnumProc p,IntPtr l);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h,System.Text.StringBuilder s,int n);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetWindowText(IntPtr h,System.Text.StringBuilder s,int n);
 [DllImport("user32.dll")] public static extern bool EnumChildWindows(IntPtr h,EnumProc p,IntPtr l);
 [DllImport("user32.dll")] public static extern int GetDlgCtrlID(IntPtr h);
 public static string Dump(uint pid) {
 var lines=new System.Collections.Generic.List<string>();
 EnumWindows((h,l)=>{uint owner;GetWindowThreadProcessId(h,out owner);if(owner==pid){var c=new System.Text.StringBuilder(256);var title=new System.Text.StringBuilder(1024);GetClassName(h,c,256);GetWindowText(h,title,1024);lines.Add(c+":"+title+" button1="+GetDlgItem(h,1));EnumChildWindows(h,(child,unused)=>{var text=new System.Text.StringBuilder(2048);GetWindowText(child,text,2048);lines.Add(GetDlgCtrlID(child)+":"+text);return true;},IntPtr.Zero);}return true;},IntPtr.Zero);
 return String.Join("; ",lines);
 }
 public static IntPtr OwnedDialog(uint pid,int button) {
  IntPtr found=IntPtr.Zero;
  EnumWindows((h,l)=>{uint owner;GetWindowThreadProcessId(h,out owner);var c=new System.Text.StringBuilder(64);GetClassName(h,c,64);if(owner==pid && c.ToString()=="#32770" && GetDlgItem(h,button)!=IntPtr.Zero){found=h;return false;}return true;},IntPtr.Zero);
  return found;
 }

 [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern IntPtr FindWindow(string c,string t);
 [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h,out uint p);
 [DllImport("user32.dll")] public static extern IntPtr GetDlgItem(IntPtr h,int id);
 [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr h,uint m,IntPtr w,IntPtr l);
 [DllImport("user32.dll")] public static extern IntPtr SendMessage(IntPtr h,uint m,IntPtr w,IntPtr l);
}
'@
$p=$null
try {
 if(Test-Path $dir){throw 'Fixture already exists; use a new test directory'}
 foreach($name in @('vm\disk.raw','guest\build-spec.json','guest\rootfs.ext4','guest\vmlinuz-linux','guest\initramfs-linux.img','runtime\bin\qemu.exe')) {
  $path=Join-Path $dir $name
  New-Item -ItemType Directory -Path (Split-Path $path) -Force | Out-Null
  [IO.File]::WriteAllText($path,'Synthetic fixture '+$name)
 }
 [IO.File]::WriteAllText("$dir\guest\build-spec.json",'{"image":{"architecture":"x86_64"}}')
 Copy-Item $Launcher "$dir\Candidate.exe"
 $p=Start-Process "$dir\Candidate.exe" -ArgumentList '-dir',$dir,'-recovery','snapshots' -RedirectStandardError "$dir\stderr.txt" -RedirectStandardOutput "$dir\stdout.txt" -PassThru
 $deadline=(Get-Date).AddSeconds(20)
 do { Start-Sleep -Milliseconds 200; $h=[SnapshotUI]::FindWindow('TryOmarchySnapshots','Try Omarchy snapshots') } while($h -eq [IntPtr]::Zero -and (Get-Date) -lt $deadline)
 if($h -eq [IntPtr]::Zero){throw 'Snapshot manager did not open'}
 foreach($id in 4100..4106) { if([SnapshotUI]::GetDlgItem($h,$id) -eq [IntPtr]::Zero){throw "Missing control $id"} }
 [SnapshotUI]::PostMessage($h,273,[IntPtr]4102,[IntPtr]::Zero)|Out-Null
 $deadline=(Get-Date).AddSeconds(30)
 $list=[SnapshotUI]::GetDlgItem($h,4100)
 do {Start-Sleep -Milliseconds 200; $count=[SnapshotUI]::SendMessage($list,395,[IntPtr]::Zero,[IntPtr]::Zero).ToInt32()} while($count -ne 1 -and (Get-Date) -lt $deadline)
 if($count -ne 1){throw 'Create did not publish and refresh the snapshot'}
 $entry=@(Get-ChildItem "$dir\checkpoints" -Directory | Where-Object Name -Match '^[a-f0-9]{32}$')
 if($entry.Count -ne 1){throw 'Expected one completed snapshot'}
 $meta=Get-Content "$($entry[0].FullName)\snapshot.json" -Raw | ConvertFrom-Json
 $hash=(Get-FileHash "$($entry[0].FullName)\vm.zip" -Algorithm SHA256).Hash.ToLower()
 if($hash -ne $meta.archiveSHA256){throw 'Archive checksum mismatch'}

 $original=[IO.File]::ReadAllText("$dir\vm\disk.raw")
 [IO.File]::WriteAllText("$dir\vm\disk.raw",'New work to retain')
 [SnapshotUI]::PostMessage($h,273,[IntPtr]4106,[IntPtr]::Zero)|Out-Null
 function Wait-OwnedDialog([int]$buttonID) {
  $deadline=(Get-Date).AddSeconds(30)
  do {
   Start-Sleep -Milliseconds 150
   $dialog=[SnapshotUI]::OwnedDialog($p.Id,$buttonID)
   [uint32]$owner=0
   if($dialog -ne [IntPtr]::Zero){[SnapshotUI]::GetWindowThreadProcessId($dialog,[ref]$owner)|Out-Null}
  } while(($dialog -eq [IntPtr]::Zero -or $owner -ne $p.Id -or [SnapshotUI]::GetDlgItem($dialog,$buttonID) -eq [IntPtr]::Zero) -and (Get-Date) -lt $deadline)
  if($owner -ne $p.Id -or [SnapshotUI]::GetDlgItem($dialog,$buttonID) -eq [IntPtr]::Zero){throw "Missing owned dialog button $buttonID. $([SnapshotUI]::Dump($p.Id))"}
  return $dialog
 }
 $confirm=Wait-OwnedDialog 6
 [SnapshotUI]::PostMessage($confirm,273,[IntPtr]6,[IntPtr]::Zero)|Out-Null
 $done=Wait-OwnedDialog 2
 if([IO.File]::ReadAllText("$dir\vm\disk.raw") -ne $original){throw 'Active rollback did not restore the disk'}
 $retained=@(Get-ChildItem $dir -Directory -Force | Where-Object Name -Like '.snapshot-rollback-*')
 if($retained.Count -ne 1 -or [IO.File]::ReadAllText("$($retained[0].FullName)\data\vm\disk.raw") -ne 'New work to retain'){throw 'Current work was not retained'}
 if(-not (Test-Path "$($retained[0].FullName)\data\Start Omarchy.lnk")){throw 'Recovery launcher missing'}
 [SnapshotUI]::PostMessage($done,273,[IntPtr]2,[IntPtr]::Zero)|Out-Null
 Start-Sleep -Milliseconds 300
 [SnapshotUI]::PostMessage($h,273,[IntPtr]2,[IntPtr]::Zero)|Out-Null
 if(-not $p.WaitForExit(10000)){throw 'Escape action failed to close window'}
 [ordered]@{result='PASS';fixture='synthetic';createAndList=$true;activeRollback=$true;retainedWork=$true;archiveChecksum=$hash;allControlsPresent=$true;cancelCommandCloses=$true}|ConvertTo-Json|Set-Content $ResultPath
} catch { $_.Exception.Message | Set-Content ($ResultPath+'.error.txt'); throw }
finally {if($p -and -not $p.HasExited){$p.Kill()}}
