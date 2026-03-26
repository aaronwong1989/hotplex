#Requires -RunAsAdministrator
# ==============================================================================
# HotPlex 一键安装脚本 (Windows PowerShell)
# Phase 2-4: 完整实现
# 支持: Windows 10/11, Windows Server 2019+
# ==============================================================================

[CmdletBinding()]
param(
    [ValidateSet("Install", "Upgrade", "Status", "Uninstall", "Purge", "")]
    [string]$Action = "",
    [string]$Version = "",
    [string]$InstallDir = "$env:ProgramFiles\HotPlex",
    [string]$ConfigDir = "$env:USERPROFILE\.hotplex",
    [switch]$SkipHealthCheck,
    [switch]$SkipAutostart,
    [switch]$SkipVerify,
    [switch]$Force,
    [switch]$NonInteractive
)

# ==============================================================================
# 全局变量
# ==============================================================================

$Script:Repo = "hrygo/hotplex"
$Script:BinaryName = "hotplexd.exe"
$Script:ServiceName = "HotPlex"
$Script:ScriptVersion = "3.0.0"
$Script:ApiBase = "https://api.github.com/repos"
$Script:DownloadBase = "https://github.com/hrygo/hotplex/releases/download"

$Script:Colors = @{
    Red=""; Green=""; Yellow=""; Cyan=""; Magenta=""; Bold=""; Dim=""; NC=""
    if ($Host.UI.SupportsVirtualTerminal) {
        $Script:Colors.Red="`e[0;31m"; $Script:Colors.Green="`e[0;32m"
        $Script:Colors.Yellow="`e[1;33m"; $Script:Colors.Cyan="`e[0;36m"
        $Script:Colors.Magenta="`e[0;35m"; $Script:Colors.Bold="`e[1m"
        $Script:Colors.Dim="`e[2m"; $Script:Colors.NC="`e[0m"
    }
}

# ==============================================================================
# 工具函数
# ==============================================================================

function Write-Banner { param([string]$Title) $l="  $($Script:Colors.Bold)===============================================================$($Script:Colors.NC)"; Write-Host ""; Write-Host $l; Write-Host "  $($Script:Colors.Cyan)$($Script:Colors.Bold)$Title$($Script:Colors.NC)"; Write-Host $l; Write-Host "" }
function Write-Info { param([string]$M) { Write-Host "$($Script:Colors.Cyan)▸$($Script:Colors.NC) $M" } }
function Write-Success { param([string]$M) { Write-Host "$($Script:Colors.Green)✓$($Script:Colors.NC) $M" } }
function Write-Warn { param([string]$M) { Write-Host "$($Script:Colors.Yellow)!$($Script:Colors.NC) $M" } }
function Write-Err { param([string]$M) { Write-Host "$($Script:Colors.Red)✗$($Script:Colors.NC) $M" } }
function Write-Debug { param([string]$M) { if ($VerbosePreference -eq "Continue") { Write-Host "$($Script:Colors.Dim)[DEBUG]$($Script:Colors.NC) $M" } } }

function Mask-Token {
    param([string]$Token, [int]$Visible = 4)
    if ([string]::IsNullOrEmpty($Token) -or $Token.Length -le ($Visible * 2)) { return "****" }
    return "$($Token.Substring(0, $Visible))...$($Token.Substring($Token.Length - $Visible))"
}

function Confirm-Action {
    param([string]$Prompt, [string]$Default = "n")
    if ($NonInteractive) { return ($Default -eq "y") }
    $choices = if ($Default -eq "y") { "[Y/n]" } else { "[y/N]" }
    Write-Host "$($Script:Colors.Bold)?$($Script:Colors.NC) $Prompt $choices: " -NoNewline
    $resp = Read-Host
    if ([string]::IsNullOrEmpty($resp)) { $resp = $Default }
    return ($resp -eq "y" -or $resp -eq "Y")
}

function Read-InputPrompt {
    param([string]$Prompt, [string]$Default = "", [bool]$Secret = $false)
    if ($NonInteractive) { return $Default }
    Write-Host "$($Script:Colors.Bold)?$($Script:Colors.NC) $Prompt" -NoNewline
    if (![string]::IsNullOrEmpty($Default)) { Write-Host " [$Default]" -NoNewline }
    Write-Host ": " -NoNewline
    if ($Secret) {
        $resp = Read-Host -AsSecureString
        $bstr = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($resp)
        $resp = [System.Runtime.InteropServices.Marshal]::PtrToStringAuto($bstr)
        [System.Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
        Write-Host ""
    } else { $resp = Read-Host }
    return if ([string]::IsNullOrEmpty($resp)) { $Default } else { $resp }
}

function Invoke-HttpGet {
    param([string]$Url)
    try {
        return (Invoke-WebRequest -Uri $Url -UserAgent "HotPlex-Installer/$($Script:ScriptVersion)" -TimeoutSec 30 -ErrorAction Stop).Content
    } catch { return $null }
}

function Test-HttpGet {
    param([string]$Url, [string]$OutputFile = "")
    try {
        if (![string]::IsNullOrEmpty($OutputFile)) {
            Invoke-WebRequest -Uri $Url -OutFile $OutputFile -UserAgent "HotPlex-Installer/$($Script:ScriptVersion)" -TimeoutSec 120 -ErrorAction Stop | Out-Null
            return $true
        } else {
            Invoke-WebRequest -Uri $Url -UserAgent "HotPlex-Installer/$($Script:ScriptVersion)" -TimeoutSec 30 -ErrorAction Stop | Out-Null
            return $true
        }
    } catch { return $false }
}

function Download-WithRetry {
    param([string]$Url, [string]$OutputPath, [int]$MaxRetries = 3)
    $retry = 0
    while ($retry -lt $MaxRetries) {
        if (Test-HttpGet -Url $Url -OutputFile $OutputPath) {
            if (Test-Path $OutputPath -PathType Leaf -and (Get-Item $OutputPath).Length -gt 0) { return $true }
        }
        $retry++
        if ($retry -lt $MaxRetries) {
            Write-Warn "下载失败，${retry}秒后重试..."
            Start-Sleep -Seconds $retry
        }
    }
    return $false
}

function Get-Platform {
    $os = $PSVersionTable.Platform
    switch ($os) {
        "Win32NT" { $os = "windows" }
        $null { $os = "windows" }
        "Unix" {
            if ($IsMacOS) { $os = "darwin" }
            elseif ($IsLinux) { $os = "linux" }
        }
    }
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        "AMD64" { $arch = "amd64" }
        "ARM64" { $arch = "arm64" }
    }
    return @{ OS = $os; Arch = $arch }
}

# ==============================================================================
# Phase 2: 端口冲突检测
# ==============================================================================

function Get-PortStatus {
    param([int]$Port)
    $conn = Get-NetTCPConnection -LocalPort $Port -ErrorAction SilentlyContinue
    if ($conn) {
        $proc = Get-Process -Id $conn[0].OwningProcess -ErrorAction SilentlyContinue
        return @{ InUse = $true; PID = $conn[0].OwningProcess; ProcessName = if ($proc) { $proc.ProcessName } else { "unknown" } }
    }
    return @{ InUse = $false }
}

function Resolve-PortConflict {
    param([int]$Port)
    $status = Get-PortStatus -Port $Port
    if (-not $status.InUse) { return $Port }
    Write-Warn "端口 ${Port} 已被占用 (PID: $($status.PID) - $($status.ProcessName))"
    Write-Host ""
    Write-Host "  请选择解决方案:"
    Write-Host "    1) 终止现有进程 (PID: $($status.PID))"
    Write-Host "    2) 使用备用端口 ($($Port + 1))"
    Write-Host "    3) 取消安装"
    Write-Host ""
    $choice = Read-InputPrompt "选择解决方案" "1"
    switch ($choice) {
        "1" {
            Write-Info "正在终止进程..."
            try {
                Stop-Process -Id $status.PID -Force -ErrorAction Stop
                Start-Sleep -Seconds 1
                if (-not (Get-PortStatus -Port $Port).InUse) { Write-Success "进程已终止"; return $Port }
            } catch {
                Write-Warn "需要管理员权限，尝试提升..."
                Start-Process powershell -Verb RunAs -ArgumentList "-Command", "Stop-Process -Id $($status.PID) -Force" -Wait
            }
            if (-not (Get-PortStatus -Port $Port).InUse) { Write-Success "进程已终止"; return $Port }
            Write-Err "无法终止进程"; return -1
        }
        "2" {
            $newPort = $Port + 1
            while ((Get-PortStatus -Port $newPort).InUse) { $newPort++ }
            Write-Info "将使用备用端口: $newPort"; return $newPort
        }
        default { Write-Err "安装已取消"; return -1 }
    }
}

# ==============================================================================
# Phase 1: SHA256 校验
# ==============================================================================

function Get-Checksums { param([string]$Version, [string]$OutFile) return Test-HttpGet -Url "$Script:DownloadBase/v${Version}/SHA256SUMS" -OutputFile $OutFile }

function Test-Checksum {
    param([string]$FilePath, [string]$ChecksumsFile)
    if (-not (Test-Path $ChecksumsFile)) { Write-Warn "校验和文件不存在，跳过验证"; return $true }
    $fileName = Split-Path $FilePath -Leaf
    $expected = $null
    Get-Content $ChecksumsFile | ForEach-Object { if ($_ -match $fileName) { $expected = ($_ -split '\s+')[0] } }
    if ([string]::IsNullOrEmpty($expected)) { Write-Warn "未找到 $fileName 的校验和，跳过验证"; return $true }
    $actual = (Get-FileHash -Path $FilePath -Algorithm SHA256).Hash.ToLower()
    $expected = $expected.ToLower()
    if ($expected -eq $actual) { return $true }
    Write-Err "SHA256 校验失败! 期望: $expected 实际: $actual"; return $false
}

# ==============================================================================
# Phase 1: Slack Token 验证
# ==============================================================================

function Test-SlackToken {
    param([string]$Token)
    if ([string]::IsNullOrEmpty($Token)) { return $false }
    try {
        $headers = @{ "Authorization" = "Bearer $Token"; "Content-Type" = "application/json" }
        $resp = Invoke-RestMethod -Uri "https://slack.com/api/auth.test" -Headers $headers -Method Post -TimeoutSec 10 -ErrorAction Stop
        return $resp.ok -eq $true
    } catch { return $false }
}

function Test-SlackTokenFormat { param([string]$Token) return $Token -match "^xox[baprs]-[a-zA-Z0-9_-]+$" -and $Token.Length -ge 20 }

# ==============================================================================
# Phase 1: 配置生成 (原子写入)
# ==============================================================================

function New-ConfigFile {
    param([string]$Path, [hashtable]$Values)
    $content = @"
# HotPlex 环境配置 - $(Get-Date -Format "yyyy-MM-dd HH:mm:ss")
HOTPLEX_PORT=$($Values.Port)
HOTPLEX_LOG_LEVEL=INFO
HOTPLEX_API_KEY=$($Values.ApiKey)
HOTPLEX_DATA_DIR=$($Values.ConfigDir)
HOTPLEX_PROVIDER_TYPE=claude-code
HOTPLEX_PROVIDER_MODEL=sonnet
HOTPLEX_SLACK_BOT_USER_ID=$($Values.BotUserId)
HOTPLEX_SLACK_BOT_TOKEN=$($Values.BotToken)
HOTPLEX_SLACK_APP_TOKEN=$($Values.AppToken)
HOTPLEX_MESSAGE_STORE_ENABLED=true
HOTPLEX_MESSAGE_STORE_TYPE=sqlite
HOTPLEX_MESSAGE_STORE_SQLITE_PATH=$($Values.ConfigDir)\chatapp_messages.db
GITHUB_TOKEN=
"@
    $dir = Split-Path $Path -Parent
    if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
    $tempFile = "$env:TEMP\hotplex-env-$([System.Guid]::NewGuid().ToString('N')).tmp"
    $content | Out-File -FilePath $tempFile -Encoding UTF8 -Force
    Move-Item -Path $tempFile -Destination $Path -Force
    $acl = Get-Acl $Path; $acl.SetAccessRuleProtection($true, $false)
    $identity = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
    $acl.AddAccessRule([System.Security.Principal.FileSystemAccessRule]::new($identity, "FullControl", "Allow"))
    $acl.AddAccessRule([System.Security.Principal.FileSystemAccessRule]::new($identity, "ReadAndExecute", "InheritOnly", "Allow"))
    Set-Acl -Path $Path -AclObject $acl
    Write-Success "已生成配置文件: $Path"
}

# ==============================================================================
# Phase 1: Windows Service 管理
# ==============================================================================

function Install-WindowsService {
    param([string]$BinaryPath, [string]$ServiceName, [string]$ConfigDir, [int]$Port)
    Write-Info "安装 Windows 服务..."
    $existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($existing) {
        if ($Force) { Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue; $null = & sc.exe delete $ServiceName 2>$null; Start-Sleep -Seconds 1 }
        else { Write-Warn "服务 $ServiceName 已存在 (使用 -Force 覆盖)"; return }
    }
    $envFile = Join-Path $ConfigDir ".env"
    $binPath = "$BinaryPath start --port $Port --env `"$envFile`""
    $null = & sc.exe create $ServiceName binPath= "$binPath" start= auto DisplayName= "HotPlex AI Agent" 2>$null
    $null = & sc.exe description $ServiceName "HotPlex AI Agent Control Plane" 2>$null
    Start-Service -Name $ServiceName -ErrorAction SilentlyContinue
    $svc = Get-Service -Name $ServiceName
    if ($svc.Status -eq "Running") { Write-Success "Windows 服务已安装并启动" } else { Write-Warn "服务安装完成但未自动启动" }
}

function Uninstall-WindowsService {
    param([string]$ServiceName)
    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if (-not $svc) { Write-Info "服务 $ServiceName 未安装"; return }
    Write-Info "停止并移除服务..."
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    $null = & sc.exe delete $ServiceName 2>$null
    Write-Success "服务已移除"
}

# ==============================================================================
# Phase 2: 交互式主菜单
# ==============================================================================

function Show-MainMenu {
    Write-Host ""
    Write-Host "$($Script:Colors.Bold)$($Script:Colors.Cyan)╭─ HotPlex 安装程序 ──────────────────────────────╮$($Script:Colors.NC)"
    Write-Host "$($Script:Colors.Bold)$($Script:Colors.Cyan)╰───────────────────────────────────────────────────╯$($Script:Colors.NC)"
    Write-Host ""
    Write-Host "  $($Script:Colors.Bold)请选择操作:$($Script:Colors.NC)"
    Write-Host ""
    Write-Host "    1) 安装 HotPlex         (Quick Start - 二进制模式)"
    Write-Host "    2) 升级 HotPlex         (保留配置，替换二进制)"
    Write-Host "    3) 卸载 HotPlex         (移除二进制，保留配置)"
    Write-Host "    4) 完全清理            (删除所有数据)"
    Write-Host "    5) 查看状态            (检查安装和运行状态)"
    Write-Host "    6) 仅配置              (运行配置向导)"
    Write-Host "    7) 退出"
    Write-Host ""
}

function Invoke-MenuChoice {
    param([string]$Choice)
    switch ($Choice) {
        "1" { Invoke-Install }
        "2" { Invoke-Upgrade }
        "3" { Invoke-Uninstall }
        "4" { Invoke-Purge }
        "5" { Invoke-Status }
        "6" { Invoke-ConfigWizard }
        "7" { Write-Host ""; Write-Info "再见!"; exit 0 }
        default { Write-Err "无效选择: $Choice" }
    }
}

# ==============================================================================
# Phase 2: 配置向导
# ==============================================================================

function Invoke-ConfigWizard {
    Write-Banner "配置向导"
    $envFile = Join-Path $ConfigDir ".env"
    if (-not (Test-Path $envFile)) { Write-Err "配置文件不存在，请先运行安装"; return }
    $content = Get-Content $envFile -Raw
    $botToken = $appToken = $botUserId = ""
    if ($content -match "HOTPLEX_SLACK_BOT_TOKEN=(.+)") { $botToken = $Matches[1].Trim() }
    if ($content -match "HOTPLEX_SLACK_APP_TOKEN=(.+)") { $appToken = $Matches[1].Trim() }
    if ($content -match "HOTPLEX_SLACK_BOT_USER_ID=(.+)") { $botUserId = $Matches[1].Trim() }
    Write-Host "  Bot Token:  $(if($botToken){"$($Script:Colors.Green)✓ $(Mask-Token -Token $botToken -Visible 6)"}else{"$($Script:Colors.Yellow)○ 未配置"})"
    Write-Host "  App Token:  $(if($appToken){"$($Script:Colors.Green)✓ $(Mask-Token -Token $appToken -Visible 6)"}else{"$($Script:Colors.Yellow)○ 未配置"})"
    Write-Host "  Bot User ID: $(if($botUserId){"$($Script:Colors.Green)✓ $botUserId"}else{"$($Script:Colors.Yellow)○ 未配置"})"
    Write-Host ""
    if (-not (Confirm-Action "是否重新配置 Slack?" "n")) { Write-Success "配置保持不变"; return }
    Write-Host ""
    $newBotToken = Read-InputPrompt "Bot Token (xoxb-...)" $botToken
    if (-not [string]::IsNullOrEmpty($newBotToken) -and (Test-SlackTokenFormat -Token $newBotToken)) {
        if (Test-SlackToken -Token $newBotToken) { Write-Success "Token 验证成功" } else { Write-Warn "Token 验证失败" }
        $content = $content -replace "HOTPLEX_SLACK_BOT_TOKEN=.+", "HOTPLEX_SLACK_BOT_TOKEN=$newBotToken"
    }
    $newAppToken = Read-InputPrompt "App Token (xapp-...)" $appToken
    if (-not [string]::IsNullOrEmpty($newAppToken)) { $content = $content -replace "HOTPLEX_SLACK_APP_TOKEN=.+", "HOTPLEX_SLACK_APP_TOKEN=$newAppToken" }
    $newBotUserId = Read-InputPrompt "Bot User ID (U... 或 B...)" $botUserId
    if (-not [string]::IsNullOrEmpty($newBotUserId)) { $content = $content -replace "HOTPLEX_SLACK_BOT_USER_ID=.+", "HOTPLEX_SLACK_BOT_USER_ID=$newBotUserId" }
    $tempFile = "$env:TEMP\hotplex-env-wizard-$([System.Guid]::NewGuid().ToString('N')).tmp"
    $content | Out-File -FilePath $tempFile -Encoding UTF8 -Force
    Move-Item -Path $tempFile -Destination $envFile -Force
    Write-Success "配置已更新: $envFile"
}

# ==============================================================================
# Phase 2-3: 安装
# ==============================================================================

function Invoke-Install {
    Write-Banner "安装 HotPlex"
    $plat = Get-Platform; $os = $plat.OS; $arch = $plat.Arch
    Write-Info "平台: $os / $arch"

    if ([string]::IsNullOrEmpty($Version)) {
        Write-Info "获取最新版本..."
        $json = Invoke-HttpGet -Url "$Script:ApiBase/$Script:Repo/releases/latest"
        if ($json) { $obj = $json | ConvertFrom-Json; $Version = $obj[0].tag_name -replace "^v", "" } else { $Version = "0.35.4" }
    }
    if (-not $Version.StartsWith("v")) { $Version = "v$Version" }
    Write-Info "目标版本: $($Script:Colors.Green)${Version}$($Script:Colors.NC)"

    $binaryPath = Join-Path $InstallDir $Script:BinaryName
    if ((Test-Path $binaryPath) -and -not $Force) {
        try {
            $current = & $binaryPath -version 2>$null | Select-Object -First 1
            if ($current) { Write-Warn "已安装版本: $current"; if (Confirm-Action "是否强制重新安装?" "n") { $Force = $true } else { return } }
        } catch { }
    }

    $port = 8080
    $portStatus = Get-PortStatus -Port $port
    if ($portStatus.InUse) { $port = Resolve-PortConflict -Port $port; if ($port -lt 0) { return } }

    $archiveName = "hotplex_$($Version.Substring(1))_${os}_${arch}"
    if ($os -eq "windows") { $archiveName += ".zip" } else { $archiveName += ".tar.gz" }
    $archiveUrl = "$Script:DownloadBase/${Version}/$archiveName"

    $tempDir = "$env:TEMP\hotplex-$([System.Guid]::NewGuid().ToString('N'))"
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    $archivePath = Join-Path $tempDir $archiveName

    Write-Info "正在下载..."
    if (-not (Download-WithRetry -Url $archiveUrl -OutputPath $archivePath)) { Write-Err "下载失败"; Remove-Item $tempDir -Recurse -Force -EA SilentlyContinue; return }
    Write-Success "下载完成"

    if (-not $SkipVerify) {
        $checksumsFile = Join-Path $tempDir "SHA256SUMS"
        if (Get-Checksums -Version $Version.Substring(1) -OutFile $checksumsFile) {
            if (-not (Test-Checksum -FilePath $archivePath -ChecksumsFile $checksumsFile)) { Remove-Item $tempDir -Recurse -Force -EA SilentlyContinue; return }
        } else { Write-Warn "无法下载校验和，跳过验证" }
    }

    Write-Info "解压..."
    $extractDir = Join-Path $tempDir "extract"
    New-Item -ItemType Directory -Path $extractDir -Force | Out-Null
    Expand-Archive -Path $archivePath -DestinationPath $extractDir -Force
    if (-not (Test-Path $InstallDir)) { New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null }

    $extractedBinary = Get-ChildItem $extractDir -Filter "*.exe" | Select-Object -First 1
    if (-not $extractedBinary) { $extractedBinary = Get-ChildItem $extractDir -Recurse -Filter "*.exe" | Select-Object -First 1 }
    if ($extractedBinary) { Copy-Item $extractedBinary.FullName $binaryPath -Force; Write-Success "已安装: $binaryPath" }
    else { Write-Err "未找到可执行文件"; Remove-Item $tempDir -Recurse -Force -EA SilentlyContinue; return }

    Write-Host ""; Write-Banner "配置 Slack 凭据"
    $botToken = Read-InputPrompt "Bot User OAuth Token (xoxb-...)" ""
    if ([string]::IsNullOrEmpty($botToken)) { Write-Err "Bot Token 不能为空"; Remove-Item $tempDir -Recurse -Force -EA SilentlyContinue; return }
    if (Test-SlackTokenFormat -Token $botToken) { if (Test-SlackToken -Token $botToken) { Write-Success "Token 验证成功" } else { Write-Warn "Token 验证失败，但继续安装" } }
    $appToken = Read-InputPrompt "App-Level Token (xapp-...)" ""
    $botUserId = Read-InputPrompt "Bot User ID (U... 或 B...)" ""
    $apiKey = [Convert]::ToBase64String([byte[]]::new(32) | ForEach-Object { Get-Random -Maximum 256 })

    Write-Host ""
    New-ConfigFile -Path (Join-Path $ConfigDir ".env") -Values @{ Port=$port; BotToken=$botToken; AppToken=$appToken; BotUserId=$botUserId; ApiKey=$apiKey; ConfigDir=$ConfigDir }

    if (-not $SkipAutostart) { Install-WindowsService -BinaryPath $binaryPath -ServiceName $Script:ServiceName -ConfigDir $ConfigDir -Port $port }

    if (-not $SkipHealthCheck) {
        Write-Info "健康检查 (超时: 15s)..."
        $healthUrl = "http://localhost:${port}/health"; $ok = $false
        for ($i = 0; $i -lt 15; $i++) {
            try { $resp = Invoke-WebRequest -Uri $healthUrl -TimeoutSec 2 -EA SilentlyContinue; if ($resp.Content -match "ok|OK|healthy") { Write-Success "健康检查通过"; $ok = $true; break } } catch { }
            Start-Sleep -Seconds 1
        }
        if (-not $ok) { Write-Warn "健康检查超时 - 服务可能仍在启动" }
    }

    Remove-Item $tempDir -Recurse -Force -EA SilentlyContinue
    Write-Host ""; Write-Success "安装完成!"; Write-Host "  二进制: $binaryPath"; Write-Host "  配置: $(Join-Path $ConfigDir ".env")"; Write-Host "  端口: $port"
}

# ==============================================================================
# Phase 3: 升级
# ==============================================================================

function Invoke-Upgrade {
    Write-Banner "升级 HotPlex"
    $binaryPath = Join-Path $InstallDir $Script:BinaryName
    if (-not (Test-Path $binaryPath)) { Write-Err "未检测到已安装的 HotPlex，请先运行安装"; return }
    try { $current = & $binaryPath -version 2>$null | Select-Object -First 1 } catch { $current = "unknown" }
    Write-Info "当前版本: $current"

    if ([string]::IsNullOrEmpty($Version)) {
        $json = Invoke-HttpGet -Url "$Script:ApiBase/$Script:Repo/releases/latest"
        if ($json) { $obj = $json | ConvertFrom-Json; $Version = $obj[0].tag_name }
    }
    if (-not $Version.StartsWith("v")) { $Version = "v$Version" }
    Write-Info "目标版本: $Version"

    $targetVersion = $Version.Substring(1)
    if ($current -match "v?([\d\.]+)") { if ($Matches[1] -eq $targetVersion) { Write-Success "已是最新版本 ($current)"; return } }

    if (-not (Confirm-Action "确认升级?" "y")) { Write-Info "升级已取消"; return }

    $svc = Get-Service -Name $Script:ServiceName -EA SilentlyContinue
    if ($svc -and $svc.Status -eq "Running") { Write-Info "停止服务..."; Stop-Service -Name $Script:ServiceName -Force }

    $plat = Get-Platform; $os = $plat.OS; $arch = $plat.Arch
    $archiveName = "hotplex_${targetVersion}_${os}_${arch}"
    if ($os -eq "windows") { $archiveName += ".zip" } else { $archiveName += ".tar.gz" }
    $archiveUrl = "$Script:DownloadBase/${Version}/$archiveName"

    $tempDir = "$env:TEMP\hotplex-upgrade-$([System.Guid]::NewGuid().ToString('N'))"
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    $archivePath = Join-Path $tempDir $archiveName

    Write-Info "正在下载 v${targetVersion}..."
    if (-not (Download-WithRetry -Url $archiveUrl -OutputPath $archivePath)) { Write-Err "下载失败"; Remove-Item $tempDir -Recurse -Force -EA SilentlyContinue; return }

    if (-not $SkipVerify) {
        $checksumsFile = Join-Path $tempDir "SHA256SUMS"
        if (Get-Checksums -Version $targetVersion -OutFile $checksumsFile) {
            if (-not (Test-Checksum -FilePath $archivePath -ChecksumsFile $checksumsFile)) { Remove-Item $tempDir -Recurse -Force -EA SilentlyContinue; return }
        }
    }

    $extractDir = Join-Path $tempDir "extract"
    New-Item -ItemType Directory -Path $extractDir -Force | Out-Null
    Expand-Archive -Path $archivePath -DestinationPath $extractDir -Force

    $extractedBinary = Get-ChildItem $extractDir -Filter "*.exe" | Select-Object -First 1
    if (-not $extractedBinary) { $extractedBinary = Get-ChildItem $extractDir -Recurse -Filter "*.exe" | Select-Object -First 1 }
    if ($extractedBinary) { Copy-Item $extractedBinary.FullName $binaryPath -Force }

    if (-not $SkipAutostart) { $svc = Get-Service -Name $Script:ServiceName -EA SilentlyContinue; if ($svc) { Write-Info "重启服务..."; Start-Service -Name $Script:ServiceName } }

    try { $newVersion = & $binaryPath -version 2>$null | Select-Object -First 1 } catch { $newVersion = "unknown" }
    Write-Success "升级完成: $newVersion"
    Remove-Item $tempDir -Recurse -Force -EA SilentlyContinue
}

# ==============================================================================
# Phase 3: 状态检查
# ==============================================================================

function Invoke-Status {
    Write-Banner "HotPlex 状态"
    $binaryPath = Join-Path $InstallDir $Script:BinaryName
    $envFile = Join-Path $ConfigDir ".env"

    Write-Host "  $($Script:Colors.Bold)二进制文件:$($Script:Colors.NC)"
    if (Test-Path $binaryPath) {
        try { $current = & $binaryPath -version 2>$null | Select-Object -First 1 } catch { $current = "unknown" }
        Write-Host "    $($Script:Colors.Green)✓$($Script:Colors.NC) 已安装: $current"
        Write-Host "    $($Script:Colors.Dim)   路径: $binaryPath$($Script:Colors.NC)"
    } else { Write-Host "    $($Script:Colors.Red)✗$($Script:Colors.NC) 未安装" }

    Write-Host ""; Write-Host "  $($Script:Colors.Bold)配置文件:$($Script:Colors.NC)"
    if (Test-Path $envFile) {
        Write-Host "    $($Script:Colors.Green)✓$($Script:Colors.NC) 已配置: $envFile"
        $content = Get-Content $envFile -Raw
        if ($content -match "HOTPLEX_SLACK_BOT_TOKEN=(.+)") { Write-Host "    $($Script:Colors.Dim)   Bot Token: $($Script:Colors.Green)$(Mask-Token -Token $Matches[1].Trim() -Visible 8)$($Script:Colors.NC)" }
        if ($content -match "HOTPLEX_PORT=(\d+)") { Write-Host "    $($Script:Colors.Dim)   端口: $($Script:Colors.Green)$($Matches[1])$($Script:Colors.NC)" }
    } else { Write-Host "    $($Script:Colors.Yellow)○$($Script:Colors.NC) 未配置" }

    Write-Host ""; Write-Host "  $($Script:Colors.Bold)服务状态:$($Script:Colors.NC)"
    $svc = Get-Service -Name $Script:ServiceName -EA SilentlyContinue
    if ($svc) {
        Write-Host "    $($Script:Colors.Green)✓$($Script:Colors.NC) 服务: $($svc.Status)"
        if ($svc.Status -eq "Running") {
            $port = 8080; if (Test-Path $envFile) { $c = Get-Content $envFile -Raw; if ($c -match "HOTPLEX_PORT=(\d+)") { $port = $Matches[1] } }
            try { $resp = Invoke-WebRequest -Uri "http://localhost:${port}/health" -TimeoutSec 3 -EA SilentlyContinue; if ($resp.Content -match "ok|OK|healthy") { Write-Host "    $($Script:Colors.Green)✓$($Script:Colors.NC) 健康检查: 通过" } else { Write-Host "    $($Script:Colors.Yellow)○$($Script:Colors.NC) 健康检查: 未通过" } } catch { Write-Host "    $($Script:Colors.Yellow)○$($Script:Colors.NC) 健康检查: 无法访问" }
        }
    } else { Write-Host "    $($Script:Colors.Yellow)○$($Script:Colors.NC) 服务未安装" }

    Write-Host ""; Write-Host "  $($Script:Colors.Bold)进程:$($Script:Colors.NC)"
    $proc = Get-Process -Name "hotplexd" -EA SilentlyContinue
    if ($proc) { Write-Host "    $($Script:Colors.Green)✓$($Script:Colors.NC) 运行中 (PID: $($proc.Id))"; Write-Host "    $($Script:Colors.Dim)   内存: $([math]::Round($proc.WorkingSet64 / 1MB, 1)) MB$($Script:Colors.NC)" }
    else { Write-Host "    $($Script:Colors.Yellow)○$($Script:Colors.NC) 未运行" }
    Write-Host ""
}

# ==============================================================================
# Phase 3: 卸载
# ==============================================================================

function Invoke-Uninstall {
    Write-Banner "卸载 HotPlex"
    $binaryPath = Join-Path $InstallDir $Script:BinaryName
    if (-not (Test-Path $binaryPath) -and -not (Test-Path $ConfigDir)) { Write-Warn "HotPlex 未安装，无需卸载"; return }
    if (-not (Confirm-Action "确认卸载 (保留配置)?" "n")) { Write-Info "卸载已取消"; return }

    $svc = Get-Service -Name $Script:ServiceName -EA SilentlyContinue
    if ($svc) { Write-Info "停止服务..."; Stop-Service -Name $Script:ServiceName -Force -EA SilentlyContinue; $null = & sc.exe delete $Script:ServiceName 2>$null }

    if (Test-Path $binaryPath) { Write-Info "删除二进制..."; Remove-Item $binaryPath -Force; Write-Success "已删除: $binaryPath" }
    Write-Success "卸载完成 (配置保留在 $ConfigDir)"
}

# ==============================================================================
# Phase 3: 完全清理
# ==============================================================================

function Invoke-Purge {
    Write-Banner "完全清理 HotPlex"
    if (-not (Test-Path (Join-Path $InstallDir $Script:

function Invoke-Purge {
    Write-Banner "完全清理 HotPlex"
    if (-not (Test-Path (Join-Path $InstallDir $Script:BinaryName)) -and -not (Test-Path $ConfigDir)) { Write-Warn "HotPlex 未安装，无需清理"; return }
    if (-not (Confirm-Action "确认完全清理 (删除所有数据)?" "n")) { Write-Info "清理已取消"; return }

    $svc = Get-Service -Name $Script:ServiceName -EA SilentlyContinue
    if ($svc) { Write-Info "停止服务..."; Stop-Service -Name $Script:ServiceName -Force -EA SilentlyContinue; $null = & sc.exe delete $Script:ServiceName 2>$null }

    $binaryPath = Join-Path $InstallDir $Script:BinaryName
    if (Test-Path $binaryPath) { Write-Info "删除二进制..."; Remove-Item $binaryPath -Force; Write-Success "已删除: $binaryPath" }
    if (Test-Path $InstallDir) { Write-Info "删除安装目录..."; Remove-Item $InstallDir -Recurse -Force; Write-Success "已删除: $InstallDir" }
    if (Test-Path $ConfigDir) { Write-Info "删除配置目录..."; Remove-Item $ConfigDir -Recurse -Force; Write-Success "已删除: $ConfigDir" }
    Write-Success "完全清理完成"
}

# ==============================================================================
# 主入口
# ==============================================================================

Write-Banner "HotPlex 安装程序 v${Script:ScriptVersion}"

# 处理 Action 参数
if (-not [string]::IsNullOrEmpty($Action)) {
    switch ($Action) {
        "Install"   { Invoke-Install }
        "Upgrade"   { Invoke-Upgrade }
        "Status"    { Invoke-Status }
        "Uninstall" { Invoke-Uninstall }
        "Purge"     { Invoke-Purge }
        default { Write-Err "未知操作: $Action"; exit 1 }
    }
    exit 0
}

# 交互模式: 显示主菜单
if ($NonInteractive -or -not $Host.UI.Interactive) {
    if (-not $SkipHealthCheck -or -not $SkipAutostart -or -not [string]::IsNullOrEmpty($Version) -or $Force) {
        Invoke-Install
    } else {
        Write-Err "非交互模式需要指定 -Action 参数"
        Write-Host "用法: .\hotplex-install.ps1 -Action Install|Upgrade|Status|Uninstall|Purge"
        exit 1
    }
    exit 0
}

# 显示主菜单循环
while ($true) {
    Show-MainMenu
    $choice = Read-InputPrompt "请选择 [1-7]" ""
    if ([string]::IsNullOrEmpty($choice)) { $choice = "1" }
    Invoke-MenuChoice -Choice $choice
}