param(
    [string]$RequestPath,
    [string]$OperationB64,
    [string]$MethodB64,
    [switch]$BackgroundUpdate,
    [string]$BackgroundRun,
    [string]$VisibleRunPayload
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$Utf8 = New-Object System.Text.UTF8Encoding($false)
[Console]::InputEncoding = $Utf8
[Console]::OutputEncoding = $Utf8
$State = Join-Path $env:USERPROFILE '.idata\server-command-runtime'
$Library = Join-Path $env:USERPROFILE '.idata\newest_testcases'
$SettingsPath = Join-Path $State 'settings.json'
$ModelPath = Join-Path $State 'model-config.json'
$UpdatePath = Join-Path $State 'test-case-update.json'
[IO.Directory]::CreateDirectory($State) | Out-Null

function Write-JsonFile([string]$Path, $Value) {
    $temporary = $Path + '.' + [guid]::NewGuid().ToString('N') + '.tmp'
    [IO.File]::WriteAllText($temporary, ($Value | ConvertTo-Json -Depth 30), $Utf8)
    Move-Item -LiteralPath $temporary -Destination $Path -Force
}

function Read-JsonFile([string]$Path, $Fallback) {
    if (Test-Path -LiteralPath $Path) {
        return (Get-Content -LiteralPath $Path -Raw -Encoding UTF8 | ConvertFrom-Json)
    }
    return $Fallback
}

function Default-Settings {
    return [ordered]@{
        projectName = 'IDATA'; releaseName = 'FangTian 1.10-1.12'
        defaultEnvironment = 'HarmonyOS'; defaultOwner = 'kouyanan 30030842'
        testCaseArchiveUrl = 'http://10.90.65.189:54322/Testcases.tar.gz'
        testCaseLibraryPath = $Library; idataExecutablePath = 'IDATA.exe'
        autoLoadDevices = $true; deviceRefreshSeconds = 30; tablePageSize = 20
    }
}

function Get-Settings {
    $settings = Default-Settings
    if (Test-Path -LiteralPath $SettingsPath) {
        $loaded = Read-JsonFile $SettingsPath $null
        foreach ($key in @($settings.Keys)) {
            if ($loaded.PSObject.Properties.Name -contains $key) { $settings[$key] = $loaded.$key }
        }
    }
    return $settings
}

function Set-ObjectValue($Object, [string]$Name, $Value) {
    if ($Object.PSObject.Properties.Name -contains $Name) { $Object.$Name = $Value }
    else { $Object | Add-Member -NotePropertyName $Name -NotePropertyValue $Value }
}

function Read-LogText([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return '' }
    $text = Get-Content -LiteralPath $Path -Raw -Encoding UTF8
    if ($text.Length -gt 2000000) { return "[Earlier output omitted from the web view; download the complete log.]`r`n" + $text.Substring($text.Length - 2000000) }
    return $text
}

function Get-UpdateStatus {
    return (Read-JsonFile $UpdatePath ([ordered]@{status='idle'; message='Ready to update the test case library.'}))
}

function Set-UpdateStatus([string]$Status, [string]$Message) {
    Write-JsonFile $UpdatePath ([ordered]@{status=$Status; message=$Message})
}

function Get-SystemExecutable([string]$Name) {
    if (-not [string]::IsNullOrWhiteSpace($env:SystemRoot)) {
        $systemPath = Join-Path $env:SystemRoot ('System32\' + $Name)
        if (Test-Path -LiteralPath $systemPath -PathType Leaf) { return $systemPath }
    }
    $command = Get-Command $Name -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($command) { return $command.Source }
    throw "Required Windows executable was not found: $Name"
}

function Get-ExternalExecutable([string]$Name) {
    $command = Get-Command $Name -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($command) { return $command.Source }
    throw "Required executable is not available in PATH: $Name"
}

function Find-TestCases($Current) {
    $root = [IO.Path]::GetFullPath([Environment]::ExpandEnvironmentVariables([string]$Current.testCaseLibraryPath))
    $mapping = Join-Path $root 'mapping.csv'
    if (-not (Test-Path -LiteralPath $mapping -PathType Leaf)) { throw "Test case mapping was not found: $mapping" }
    $files = @{}
    Get-ChildItem -LiteralPath $root -Recurse -File -Filter '*.py' | Where-Object Name -ne '__init__.py' | ForEach-Object { $files[$_.BaseName] = $_ }
    $cases = @()
    foreach ($row in @(Import-Csv -LiteralPath $mapping -Encoding UTF8)) {
        $number = [string]$row.'用例_编号'
        if ([string]::IsNullOrWhiteSpace($number) -or -not $files.ContainsKey($number.Trim())) { continue }
        $number = $number.Trim(); $file = $files[$number]
        $relative = $file.FullName.Substring($root.TrimEnd('\').Length).TrimStart('\').Replace('\','/')
        if ($relative.EndsWith('.py')) { $relative = $relative.Substring(0, $relative.Length - 3) }
        $title = if ([string]::IsNullOrWhiteSpace([string]$row.'用例_名称')) {$number} else {[string]$row.'用例_名称'}
        $cases += [ordered]@{
            id=[string]($cases.Count + 1); title=$title
            executionName=$number; path=$relative; moduleName=[string]$row.'模块_名称'; moduleCode=[string]$row.'模块_编号'
            applicationName=[string]$row.'应用_名称'; applicationCode=[string]$row.'应用_编号'
            updated=$file.LastWriteTime.ToString('yyyy-MM-ddTHH:mm:sszzz'); category='Standard'
        }
    }
    return [ordered]@{testCases=@($cases); mappingPath=$mapping}
}

function Install-TestCases {
    try {
        $current = Get-Settings
        $url = [uri]([string]$current.testCaseArchiveUrl).Trim()
        if ($url.Scheme -notin @('http','https') -or -not [string]::IsNullOrEmpty($url.UserInfo)) { throw 'Set a valid HTTP or HTTPS test case archive URL in Settings.' }
        $parent = Split-Path -Parent $Library; [IO.Directory]::CreateDirectory($parent) | Out-Null
        $stage = Join-Path $parent ('.testcases-' + [guid]::NewGuid().ToString('N')); [IO.Directory]::CreateDirectory($stage) | Out-Null
        $archive = Join-Path $stage 'Testcases.tar.gz'; $extracted = Join-Path $stage 'extracted'
        Set-UpdateStatus 'running' 'Downloading test case archive on the execution PC…'
        $curl = Get-SystemExecutable 'curl.exe'
        $tar = Get-SystemExecutable 'tar.exe'
        & $curl --fail --location --silent --show-error --connect-timeout 30 --max-time 600 --output $archive $url.AbsoluteUri
        if ($LASTEXITCODE -ne 0) { throw 'Archive download failed. Check the URL and network connection.' }
        if (-not (Test-Path -LiteralPath $archive -PathType Leaf) -or (Get-Item -LiteralPath $archive).Length -gt 2147483648) { throw 'The test case archive is missing or exceeds the 2 GB limit.' }
        Set-UpdateStatus 'running' 'Extracting and validating test cases…'
        $entries = @(& $tar -tzf $archive); if ($LASTEXITCODE -ne 0) { throw 'The test case archive could not be read.' }
        if ($entries.Count -gt 100000 -or @($entries | Where-Object { $_ -match '(^[/\\])|(^|[/\\])\.\.([/\\]|$)|:' }).Count) { throw 'The archive contains an unsafe path or too many files.' }
        [IO.Directory]::CreateDirectory($extracted) | Out-Null
        & $tar -xzf $archive -C $extracted; if ($LASTEXITCODE -ne 0) { throw 'The test case archive could not be extracted.' }
        $mappings = @(Get-ChildItem -LiteralPath $extracted -Recurse -File -Filter 'mapping.csv')
        if ($mappings.Count -ne 1) { throw 'The archive must contain exactly one test case mapping CSV.' }
        $source = $mappings[0].Directory.FullName
        $check = Find-TestCases ([pscustomobject]@{testCaseLibraryPath=$source})
        if (@($check.testCases).Count -eq 0) { throw 'The archive contains no mapped test cases.' }
        $backup = Join-Path $stage 'previous'
        if (Test-Path -LiteralPath $Library) { Move-Item -LiteralPath $Library -Destination $backup }
        try { Move-Item -LiteralPath $source -Destination $Library }
        catch { if (Test-Path -LiteralPath $Library) { Remove-Item -LiteralPath $Library -Recurse -Force }; if (Test-Path -LiteralPath $backup) { Move-Item -LiteralPath $backup -Destination $Library }; throw }
        $current.testCaseLibraryPath = $Library; Write-JsonFile $SettingsPath $current
        Set-UpdateStatus 'complete' 'Test case library updated successfully.'
    } catch {
        Set-UpdateStatus 'failed' $_.Exception.Message
    } finally {
        if ($stage -and (Test-Path -LiteralPath $stage)) { Remove-Item -LiteralPath $stage -Recurse -Force -ErrorAction SilentlyContinue }
    }
}

function Start-BackgroundUpdate {
    $worker = Join-Path $State 'update-worker.ps1'
    Copy-Item -LiteralPath $PSCommandPath -Destination $worker -Force
    $escaped = $worker.Replace("'", "''")
    $command = "& '$escaped' -BackgroundUpdate"
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($command))
    Start-Process -FilePath (Join-Path $PSHOME 'powershell.exe') -ArgumentList @('-NoLogo','-NoProfile','-NonInteractive','-ExecutionPolicy','Bypass','-EncodedCommand',$encoded) -WindowStyle Hidden | Out-Null
}

function Get-IDATAPath($Current) {
    $raw = ([string]$Current.idataExecutablePath).Trim()
    if ([string]::IsNullOrWhiteSpace($raw)) { throw 'Set the IDATA executable path in Settings.' }
    if ([IO.Path]::IsPathRooted($raw)) { return [IO.Path]::GetFullPath($raw) }
    if ($raw.Replace('\','/').ToLower() -eq '../idata.exe') { $raw = 'IDATA.exe' }
    if ([string]::IsNullOrWhiteSpace($env:IDATA_CLIENT_EXECUTABLE_DIRECTORY)) { throw 'The Client executable directory was not provided.' }
    return [IO.Path]::GetFullPath((Join-Path $env:IDATA_CLIENT_EXECUTABLE_DIRECTORY $raw))
}

function Get-RunPath([string]$RunID) {
    if ($RunID -notmatch '^TR-[0-9]+$') { throw 'Invalid test run ID.' }
    $runs = Join-Path $State 'runs'; [IO.Directory]::CreateDirectory($runs) | Out-Null
    return (Join-Path $runs ($RunID + '.json'))
}

function Serialize-Run($Run) {
    $started = @($Run.started); $finished = @($started | Where-Object { $_.result -notin @('Pending','Running') })
    $failed = @($finished | Where-Object result -eq 'Failed').Count
    $interrupted = @($finished | Where-Object result -eq 'Interrupted').Count
    $status = if ($Run.stopRequested) {'Interrupted'} elseif ($finished.Count -lt $started.Count) {'Running'} elseif ($interrupted) {'Interrupted'} elseif ($failed) {'Failed'} else {'Completed'}
    $passed = @($finished | Where-Object result -eq 'Passed').Count
    $progress = if ($started.Count) {[math]::Round($finished.Count/$started.Count*100)} else {0}
    return [ordered]@{
        id=$Run.id; title=$Run.title; device=$Run.device; inspectionMode=$Run.inspectionMode; startedAt=$Run.startedAt; status=$status
        runningProcesses=($started.Count-$finished.Count); totalProcesses=$started.Count; executedProcesses=$finished.Count
        passedProcesses=$passed; failedProcesses=$failed; interruptedProcesses=$interrupted
        progress=$progress
        consoleOutput=(@($started | ForEach-Object {[string]$_.consoleOutput}) -join "`n`n"); started=$started
    }
}

function Start-BackgroundRun([string]$RunID) {
    $worker = Join-Path $State ($RunID + '-worker.ps1')
    Copy-Item -LiteralPath $PSCommandPath -Destination $worker -Force
    $escapedWorker = $worker.Replace("'", "''"); $escapedRun = $RunID.Replace("'", "''")
    $command = "& '$escapedWorker' -BackgroundRun '$escapedRun'"
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($command))
    Start-Process -FilePath (Join-Path $PSHOME 'powershell.exe') -ArgumentList @('-NoLogo','-NoProfile','-NonInteractive','-ExecutionPolicy','Bypass','-EncodedCommand',$encoded) -WindowStyle Hidden | Out-Null
}

function Execute-VisibleRun([string]$Payload) {
    # Native programs write legitimate diagnostics to stderr. Windows PowerShell
    # 5.1 must not promote those redirected lines to terminating errors.
    $ErrorActionPreference = 'Continue'
    $record = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($Payload)) | ConvertFrom-Json
    $logPath = [string]$record.logPath
    $statusPath = [string]$record.statusPath
    $logParent = Split-Path -Parent $logPath; [IO.Directory]::CreateDirectory($logParent) | Out-Null
    $heading = @(
        'IDATA test execution log'
        "Started: $([DateTimeOffset]::Now.ToString('yyyy-MM-ddTHH:mm:ss.fffzzz'))"
        "Run ID: $($record.runID)"
        "Case ID: $($record.caseID)"
        "Case: $($record.executionName)"
        "Device: $($record.device)"
        "Computer: $env:COMPUTERNAME"
        "User: $env:USERNAME"
        "Windows: $([Environment]::OSVersion.VersionString)"
        "PowerShell: $($PSVersionTable.PSVersion)"
        "Working directory: $($record.libraryPath)"
        "IDATA executable: $($record.idataPath)"
        "Runner: $($record.runnerPath)"
        "Command: $($record.command)"
        "Log file: $logPath"
        '------------------------------------------------------------------------'
        ''
    ) -join "`r`n"
    [IO.File]::WriteAllText($logPath, $heading, $Utf8)
    [Console]::Out.Write($heading)
    $exitCode = -1; $failure = ''
    try {
        Set-Location -LiteralPath ([string]$record.libraryPath)
        & ([string]$record.idataPath) cli bundle run --path ([string]$record.runnerPath) -- ([string]$record.executionName) ([string]$record.inspectionMode) ([string]$record.device) 2>&1 | ForEach-Object {
            $line = [string]$_
            [Console]::Out.WriteLine($line)
            [IO.File]::AppendAllText($logPath, $line + "`r`n", $Utf8)
        }
        $exitCode = $LASTEXITCODE
    } catch {
        $failure = ($_ | Out-String).Trim()
        [Console]::Error.WriteLine($failure)
        [IO.File]::AppendAllText($logPath, $failure + "`r`n", $Utf8)
    } finally {
        $completion = "`r`nFinished: $([DateTimeOffset]::Now.ToString('yyyy-MM-ddTHH:mm:ss.fffzzz'))`r`nExit code: $exitCode`r`n"
        [Console]::Out.Write($completion)
        [IO.File]::AppendAllText($logPath, $completion, $Utf8)
        Write-JsonFile $statusPath ([ordered]@{exitCode=$exitCode; error=$failure})
    }
    [Console]::Out.WriteLine("日志已保存：$logPath")
    [Console]::Out.WriteLine('测试已结束，窗口将自动关闭。')
}

function Execute-Run([string]$RunID) {
    $path = Get-RunPath $RunID; $run = Read-JsonFile $path $null
    $current = Get-Settings; $idata = Get-IDATAPath $current; $runner = Join-Path ([string]$run.libraryPath) 'run_testcase.py'
    $logs = Join-Path $State ('logs\' + $RunID); [IO.Directory]::CreateDirectory($logs) | Out-Null
    foreach ($item in @($run.started)) {
        $latest = Read-JsonFile $path $null
        if ($latest.stopRequested) { Set-ObjectValue $run 'stopRequested' $true; Set-ObjectValue $item 'result' 'Interrupted'; Set-ObjectValue $item 'interruptionMessage' 'The test run was closed manually.'; Write-JsonFile $path $run; continue }
        $caseID = ([string]$item.testCase) -replace '[^A-Za-z0-9._-]', '_'
        $logPath = Join-Path $logs ($caseID + '.log'); $statusPath = Join-Path $logs ($caseID + '.status.json')
        Set-ObjectValue $item 'logPath' $logPath
        Set-ObjectValue $item 'result' 'Running'; Write-JsonFile $path $run
        try {
            $payloadObject = [ordered]@{
                runID=$RunID; caseID=[string]$item.testCase; executionName=[string]$item.executionName; device=[string]$run.device
                inspectionMode=[int]$run.inspectionMode; libraryPath=[string]$run.libraryPath; idataPath=$idata; runnerPath=$runner
                command=[string]$item.command; logPath=$logPath; statusPath=$statusPath
            }
            $payload = [Convert]::ToBase64String($Utf8.GetBytes(($payloadObject | ConvertTo-Json -Depth 10 -Compress)))
            $escapedWorker = $PSCommandPath.Replace("'", "''")
            $launch = "& '$escapedWorker' -VisibleRunPayload '$payload'"
            $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($launch))
            $process = Start-Process -FilePath (Join-Path $PSHOME 'powershell.exe') -ArgumentList @('-NoLogo','-NoProfile','-ExecutionPolicy','Bypass','-EncodedCommand',$encoded) -WorkingDirectory ([string]$run.libraryPath) -WindowStyle Normal -PassThru
            Set-ObjectValue $item 'processId' $process.Id; Write-JsonFile $path $run
            while (-not (Test-Path -LiteralPath $statusPath -PathType Leaf)) {
                $latest = Read-JsonFile $path $null
                if ($latest.stopRequested) {
                    Set-ObjectValue $run 'stopRequested' $true
                    try { $taskkill = Get-SystemExecutable 'taskkill.exe'; & $taskkill /PID ([string]$process.Id) /T /F 2>$null | Out-Null } catch { }
                    $message = 'The test run was closed manually.'
                    [IO.File]::AppendAllText($logPath, "`r`nExecution interrupted: $message`r`n", $Utf8)
                    Write-JsonFile $statusPath ([ordered]@{exitCode=$null; interrupted=$true; error=$message})
                    break
                }
                $process.Refresh()
                if ($process.HasExited) {
                    $message = 'The visible test window closed before it wrote a completion status.'
                    if (-not (Test-Path -LiteralPath $logPath)) { [IO.File]::WriteAllText($logPath, $message + "`r`n", $Utf8) }
                    else { [IO.File]::AppendAllText($logPath, "`r`n$message`r`n", $Utf8) }
                    Write-JsonFile $statusPath ([ordered]@{exitCode=-1; error=$message})
                    break
                }
                Set-ObjectValue $item 'consoleOutput' (Read-LogText $logPath); Write-JsonFile $path $run
                Start-Sleep -Milliseconds 250
            }
            $status = Read-JsonFile $statusPath ([pscustomobject]@{exitCode=-1; error='The test status file could not be read.'})
            $output = Read-LogText $logPath
            $code = $status.exitCode
            $result = if ($code -eq 0) {'Passed'} else {'Failed'}
            $latest = Read-JsonFile $path $null
            if ($status.interrupted -or $latest.stopRequested) { Set-ObjectValue $run 'stopRequested' $true; $result = 'Interrupted' }
            Set-ObjectValue $item 'result' $result
            Set-ObjectValue $item 'exitCode' $code; Set-ObjectValue $item 'consoleOutput' $output
            if ($status.error) { Set-ObjectValue $item 'error' ([string]$status.error) }
            if ($output -match '(?im)(?:report|report path|报告路径)\s*[:：]\s*(.+?\.html?)\s*$') { Set-ObjectValue $item 'reportLocation' $matches[1].Trim().Trim('"'); Set-ObjectValue $item 'reportUrl' 'available' }
        } catch {
            $failure = ($_ | Out-String).Trim()
            if (-not (Test-Path -LiteralPath $logPath)) { [IO.File]::WriteAllText($logPath, $failure + "`r`n", $Utf8) }
            else { [IO.File]::AppendAllText($logPath, "`r`n$failure`r`n", $Utf8) }
            Set-ObjectValue $item 'result' 'Failed'; Set-ObjectValue $item 'exitCode' -1; Set-ObjectValue $item 'error' $failure; Set-ObjectValue $item 'consoleOutput' (Read-LogText $logPath)
        }
        Write-JsonFile $path $run
    }
}

function Stop-Run([string]$RunID) {
    $path = Get-RunPath $RunID
    $run = Read-JsonFile $path $null
    if (-not $run) { throw 'Test run was not found.' }
    Set-ObjectValue $run 'stopRequested' $true
    $message = 'The test run was closed manually.'
    $logs = Join-Path $State ('logs\' + $RunID); [IO.Directory]::CreateDirectory($logs) | Out-Null
    $active = @($run.started | Where-Object { [string]$_.result -in @('Pending','Running') })
    foreach ($item in $active) {
        $caseID = ([string]$item.testCase) -replace '[^A-Za-z0-9._-]', '_'
        $logPath = if ($item.logPath) {[string]$item.logPath} else {Join-Path $logs ($caseID + '.log')}
        Set-ObjectValue $item 'logPath' $logPath
        Set-ObjectValue $item 'result' 'Interrupted'
        Set-ObjectValue $item 'exitCode' $null
        Set-ObjectValue $item 'error' $message
        Set-ObjectValue $item 'interruptionMessage' $message
    }
    # Persist the stopped state before attempting any process or log cleanup.
    Write-JsonFile $path $run
    foreach ($item in $active) {
        if ($item.processId) {
            try {
                $taskkill = Get-SystemExecutable 'taskkill.exe'
                & $taskkill /PID ([string]$item.processId) /T /F 2>$null | Out-Null
            } catch {
                # Process cleanup is best effort. The persisted run state above is authoritative.
            }
        }
        try {
            $logPath = [string]$item.logPath
            $logParent = Split-Path -Parent $logPath; [IO.Directory]::CreateDirectory($logParent) | Out-Null
            [IO.File]::AppendAllText($logPath, "`r`nExecution interrupted: $message`r`n", $Utf8)
            Set-ObjectValue $item 'consoleOutput' (Read-LogText $logPath)
        } catch { }
    }
    Write-JsonFile $path $run
    return (Serialize-Run $run)
}

function Handle-Request([string]$Operation, [string]$Method, $Body) {
    if ($Operation -eq 'settings') {
        $current = Get-Settings
        if ($Method -eq 'PUT') {
            $incoming = if ($Body.PSObject.Properties.Name -contains 'settings') {$Body.settings} else {$Body}
            foreach ($key in @($current.Keys)) { if ($incoming.PSObject.Properties.Name -contains $key) { $current[$key] = $incoming.$key } }
            Write-JsonFile $SettingsPath $current
        }
        return [ordered]@{settings=$current; networkZone='blue'; testCaseUpdateCommand=''}
    }
    if ($Operation -eq 'model-config') {
        if ($Method -eq 'PUT') { $value = if ($Body.PSObject.Properties.Name -contains 'modelConfig') {$Body.modelConfig} else {$Body}; Write-JsonFile $ModelPath $value }
        return [ordered]@{modelConfig=(Read-JsonFile $ModelPath ([ordered]@{api_base=''; api_key=''; model_name=''}))}
    }
    if ($Operation -eq 'devices') {
        try { $hdc = Get-ExternalExecutable 'hdc.exe'; $output = @(& $hdc list targets -v 2>&1); $code = $LASTEXITCODE }
        catch { return [ordered]@{devices=@(); error=$_.Exception.Message} }
        if ($code -ne 0) {
            $detail = ($output | ForEach-Object { [string]$_ }) -join "`n"
            if ([string]::IsNullOrWhiteSpace($detail)) { $detail = "exit code $code" }
            return [ordered]@{devices=@(); error="HDC device search failed: $detail"}
        }
        $devices = @()
        $detailCommand = 'param get const.product.model; param get const.product.name; param get const.product.os.dist.version; param get const.product.devicetype'
        foreach ($line in $output) {
            $columns = @(([string]$line).Trim() -split '\s+')
            if ($columns.Count -lt 3 -or $columns[0].ToLower() -in @('empty','[empty]','')) { continue }
            $targetID = [string]$columns[0]
            $status = [string]$columns[2]
            if ($status.ToLower() -ne 'connected' -or $targetID -notmatch '^[A-Za-z0-9._:-]+$') { continue }
            $values = @()
            try {
                $detailOutput = @(& $hdc -t $targetID shell $detailCommand 2>$null)
                if ($LASTEXITCODE -eq 0) { $values = @($detailOutput | ForEach-Object { ([string]$_).Trim() }) }
            } catch { $values = @() }
            $devices += [ordered]@{
                id=$targetID; status=$status
                model=$(if ($values.Count -gt 0) {$values[0]} else {''})
                name=$(if ($values.Count -gt 1) {$values[1]} else {''})
                osVersion=$(if ($values.Count -gt 2) {$values[2]} else {''})
                deviceType=$(if ($values.Count -gt 3) {$values[3]} else {''})
            }
        }
        return [ordered]@{devices=@($devices); error=$null}
    }
    if ($Operation -eq 'test-cases') { return (Find-TestCases (Get-Settings)) }
    if ($Operation -eq 'test-cases/update') {
        $status = Get-UpdateStatus
        if ($Method -eq 'POST' -and $status.status -ne 'running') { Set-UpdateStatus 'running' 'Preparing test case update…'; Start-BackgroundUpdate; $status = Get-UpdateStatus }
        return $status
    }
    if ($Operation -eq 'test-runs' -and $Method -eq 'GET') {
        $runsPath = Join-Path $State 'runs'; $runs = @()
        if (Test-Path -LiteralPath $runsPath) { $runs = @(Get-ChildItem -LiteralPath $runsPath -File -Filter 'TR-*.json' | ForEach-Object { Serialize-Run (Read-JsonFile $_.FullName $null) } | Sort-Object startedAt -Descending) }
        return [ordered]@{testRuns=$runs}
    }
    if ($Operation -eq 'test-runs' -and $Method -eq 'POST') {
        $selected = @($Body.testCases); $mode = [int]$Body.inspectionMode; $device = ([string]$Body.device).Trim()
        if (-not $selected.Count -or $mode -notin @(0,1,2) -or [string]::IsNullOrWhiteSpace($device)) { throw 'Select test cases, a device, and a valid inspection mode.' }
        $current = Get-Settings; $discovered = Find-TestCases $current; $available = @{}
        foreach ($case in @($discovered.testCases)) { $available[[string]$case.id] = $case }
        $idata = Get-IDATAPath $current; $runner = Join-Path ([string]$current.testCaseLibraryPath) 'run_testcase.py'
        if (-not (Test-Path -LiteralPath $idata -PathType Leaf) -or -not (Test-Path -LiteralPath $runner -PathType Leaf)) { throw 'IDATA.exe or run_testcase.py was not found.' }
        $runID = 'TR-' + [string][math]::Floor(([DateTime]::UtcNow - [DateTime]'1970-01-01').TotalMilliseconds); $started = @()
        foreach ($caseID in @($selected | Select-Object -Unique)) {
            if (-not $available.ContainsKey([string]$caseID)) { throw "Unknown test case selection: $caseID" }
            $case = $available[[string]$caseID]
            $started += [ordered]@{testCase=[string]$caseID; testCaseName=$case.executionName; executionName=$case.executionName; inspectionMode=$mode; processId=$null; command="$idata cli bundle run --path $runner -- $($case.executionName) $mode $device"; result='Pending'; consoleOutput=''; exitCode=$null; reportUrl=$null; reportLocation=$null; checks=@()}
        }
        $run = [ordered]@{id=$runID; title=[string]$Body.name; device=$device; inspectionMode=$mode; startedAt=[DateTimeOffset]::Now.ToString('yyyy-MM-ddTHH:mm:sszzz'); libraryPath=[string]$current.testCaseLibraryPath; stopRequested=$false; started=$started}
        Write-JsonFile (Get-RunPath $runID) $run; Start-BackgroundRun $runID
        return (Serialize-Run $run)
    }
    if ($Operation -match '^test-runs/(TR-[0-9]+)/close$') { return (Stop-Run $matches[1]) }
    if ($Operation -match '^test-runs/(TR-[0-9]+)/reports/([^/]+)/content$') {
        $run=Read-JsonFile (Get-RunPath $matches[1]) $null; $caseID=$matches[2]; $item=@($run.started | Where-Object {[string]$_.testCase -eq $caseID})[0]
        if (-not $item -or -not (Test-Path -LiteralPath ([string]$item.reportLocation) -PathType Leaf)) { throw 'Test report was not found.' }
        $file=Get-Item -LiteralPath ([string]$item.reportLocation); if ($file.Length -gt 8388608) { throw 'Test report exceeds the viewing limit.' }
        return [ordered]@{contentBase64=[Convert]::ToBase64String([IO.File]::ReadAllBytes($file.FullName))}
    }
    if ($Operation -match '^test-runs/(TR-[0-9]+)/logs/([^/]+)/content$') {
        $run=Read-JsonFile (Get-RunPath $matches[1]) $null; $caseID=$matches[2]; $item=@($run.started | Where-Object {[string]$_.testCase -eq $caseID})[0]
        if (-not $item -or -not (Test-Path -LiteralPath ([string]$item.logPath) -PathType Leaf)) { throw 'Test execution log was not found.' }
        $file=Get-Item -LiteralPath ([string]$item.logPath); if ($file.Length -gt 8388608) { throw 'Test execution log exceeds the download limit.' }
        return [ordered]@{contentBase64=[Convert]::ToBase64String([IO.File]::ReadAllBytes($file.FullName))}
    }
    throw "Unsupported IDATA operation: $Method $Operation"
}

if ($BackgroundUpdate) { Install-TestCases; exit 0 }
if ($BackgroundRun) { Execute-Run $BackgroundRun; exit 0 }
if ($VisibleRunPayload) { Execute-VisibleRun $VisibleRunPayload; exit 0 }

try {
    $operation = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($OperationB64))
    $method = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($MethodB64))
    $bodyText = if ($RequestPath -and (Test-Path -LiteralPath $RequestPath)) { Get-Content -LiteralPath $RequestPath -Raw -Encoding UTF8 } else { '' }
    $body = if ([string]::IsNullOrWhiteSpace($bodyText)) {[pscustomobject]@{}} else {$bodyText | ConvertFrom-Json}
    $response = [ordered]@{ok=$true; data=(Handle-Request $operation $method $body)}
} catch {
    $response = [ordered]@{ok=$false; error=$_.Exception.Message}
}
$responseBytes = $Utf8.GetBytes(($response | ConvertTo-Json -Depth 30 -Compress))
[Console]::Out.WriteLine('__IDATA_SERVER_RESPONSE__' + [Convert]::ToBase64String($responseBytes))
