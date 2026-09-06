param([Parameter(Mandatory)][ValidateSet('Setup','Cleanup')][string]$Mode)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
# Never run on developer hosts or self-hosted runners. No existing disk is selected.
if ($env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_ENVIRONMENT -ne 'github-hosted' -or
    $env:RUNNER_OS -ne 'Windows' -or $env:RUNNER_ARCH -ne 'ARM64' -or
    [Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString() -ne 'Arm64') {
    throw 'Disposable GitHub-hosted native Windows ARM64 runner required'
}
$tempRoot = [IO.Path]::GetFullPath($env:RUNNER_TEMP).TrimEnd('\') + '\'
if ($Mode -eq 'Setup') {
    $fixture = Join-Path $tempRoot ('uap-arm64-volume-' + [guid]::NewGuid().ToString('N'))
    # New-Item fails on a collision; record this unique container before disk work.
    New-Item -ItemType Directory -Path $fixture | Out-Null
    "AUTHORING_VOLUME_FIXTURE=$fixture" | Out-File $env:GITHUB_ENV -Append -Encoding utf8
} else {
    if (-not $env:AUTHORING_VOLUME_FIXTURE) { return }
    $fixture = [IO.Path]::GetFullPath($env:AUTHORING_VOLUME_FIXTURE)
}
if ([IO.Path]::GetDirectoryName($fixture) + '\' -ne $tempRoot -or
    [IO.Path]::GetFileName($fixture) -notmatch '^uap-arm64-volume-[0-9a-f]{32}$' -or
    ((Get-Item -LiteralPath $fixture).Attributes -band [IO.FileAttributes]::ReparsePoint)) {
    throw 'Fixture container identity refused'
}
$vhd = Join-Path $fixture 'fresh.vhdx'
$journal = Join-Path $fixture 'ownership.json'
if ($vhd -match '["\r\n]') { throw 'Unsafe diskpart path encoding' }
if ($Mode -eq 'Cleanup') {
    if (-not (Test-Path -LiteralPath $journal)) { return } # Setup never reached creation.
    $owner = Get-Content -Raw -LiteralPath $journal | ConvertFrom-Json
    if ($owner.path -ne $vhd -or $owner.run -ne $env:GITHUB_RUN_ID -or $owner.attempt -ne $env:GITHUB_RUN_ATTEMPT) {
        throw 'VHD ownership journal mismatch'
    }
    if (Test-Path -LiteralPath $vhd) {
        # A partially created/corrupt file may be unqueryable: fail closed and
        # retain it for runner disposal, never guess an existing disk number.
        $image = Get-DiskImage -ImagePath $vhd
        if ($image.Attached) { Dismount-DiskImage -ImagePath $vhd }
        if ((Get-DiskImage -ImagePath $vhd).Attached) { throw 'VHD remains attached' }
        Remove-Item -LiteralPath $vhd
    }
    'Detached and removed only owned fresh.vhdx' | Set-Content (Join-Path $fixture 'cleanup.txt')
    return
}
foreach ($command in @('diskpart.exe','Get-DiskImage','Mount-DiskImage','Dismount-DiskImage','Get-Disk',
    'Initialize-Disk','New-Partition','Format-Volume','Get-Volume')) { Get-Command $command | Out-Null }
if (Test-Path -LiteralPath $vhd) { throw 'Fresh VHD path already exists' }
@{path=$vhd; run=$env:GITHUB_RUN_ID; attempt=$env:GITHUB_RUN_ATTEMPT} |
    ConvertTo-Json | Set-Content -LiteralPath $journal
# diskpart creates this one new file only: no select disk, clean, online, or format.
$create = Join-Path $fixture 'create.txt'
"create vdisk file=`"$vhd`" maximum=256 type=expandable`nexit" | Set-Content -LiteralPath $create -Encoding ascii
& diskpart.exe /s $create | Out-File (Join-Path $fixture 'create.log')
if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $vhd)) { throw 'Fresh VHDX creation failed' }
$image = Mount-DiskImage -ImagePath $vhd -StorageType VHD -Access ReadWrite -PassThru
$disks = @($image | Get-Disk)
if ($disks.Count -ne 1) { throw 'Fresh image must map to exactly one disk' }
$disk = $disks[0]
$system = Get-Partition -DriveLetter C | Get-Disk
if ($disk.IsBoot -or $disk.IsSystem -or $disk.Number -eq $system.Number -or
    $disk.PartitionStyle -ne 'RAW' -or $disk.Size -ne 256MB -or -not $disk.UniqueId) {
    throw 'Fresh disk identity/RAW/size/system exclusion failed'
}
$diskId = $disk.UniqueId
function Assert-OwnedDisk {
    $current = @((Get-DiskImage -ImagePath $vhd) | Get-Disk)
    if ($current.Count -ne 1 -or $current[0].UniqueId -ne $diskId -or
        $current[0].Number -ne $disk.Number -or $current[0].IsBoot -or $current[0].IsSystem) {
        throw 'Fresh image disk identity changed'
    }
}
Assert-OwnedDisk
Initialize-Disk -InputObject $disk -PartitionStyle GPT | Out-Null
$used = @((Get-Volume).DriveLetter) + @((Get-PSDrive -PSProvider FileSystem).Name)
$free = @([char[]](90..68) | Where-Object { $_.ToString() -notin $used })
if ($free.Count -eq 0) { throw 'No free drive letter' }
$letter = $free[0]
Assert-OwnedDisk
$partition = New-Partition -InputObject ((Get-DiskImage -ImagePath $vhd) | Get-Disk) -UseMaximumSize -DriveLetter $letter
Assert-OwnedDisk
if ($partition.DiskNumber -ne $disk.Number -or $partition.DriveLetter -ne $letter) { throw 'New partition identity mismatch' }
$volume = Format-Volume -Partition $partition -FileSystem NTFS -NewFileSystemLabel UAPDisposable -Confirm:$false
$systemVolume = Get-Volume -DriveLetter C
if ($volume.FileSystem -ne 'NTFS' -or $volume.DriveType -ne 'Fixed' -or
    $volume.UniqueId -notmatch '^\\\\\?\\Volume\{[0-9a-f-]+\}\\$' -or
    $volume.UniqueId -eq $systemVolume.UniqueId) { throw 'Distinct fixed NTFS GUID proof failed' }
@{image=$vhd; disk_unique_id=$diskId; disk_number=$disk.Number; drive=$letter.ToString();
  volume_guid=$volume.UniqueId; c_volume_guid=$systemVolume.UniqueId; filesystem=$volume.FileSystem;
  invariant='Distinct filesystem identities, not separate physical hardware'} |
    ConvertTo-Json | Set-Content (Join-Path $fixture 'volume-proof.json')
# Existing required tests discover this fixed drive and verify GUID identity.
Get-Content (Join-Path $fixture 'volume-proof.json')
