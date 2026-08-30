Get-Process pythonw -ErrorAction SilentlyContinue | Where-Object { $_.Path -like "*Python314*" } | ForEach-Object { 
    # Check cmdline via WMI
    $cmd = (Get-CimInstance Win32_Process -Filter "ProcessId=$($_.Id)").CommandLine
    if ($cmd -like "*monitor.py*") { 
        Write-Host "Killing $($_.Id) $cmd"
        Stop-Process -Id $_.Id -Force 
    }
}
Get-Process pythonw -ErrorAction SilentlyContinue | Format-Table Id, ProcessName
