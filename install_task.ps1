$action = New-ScheduledTaskAction -Execute 'D:\codes\dev\winsched\winsched.exe' -Argument 'run'
$trigger = New-ScheduledTaskTrigger -AtLogon -User 'Reechard'
$principal = New-ScheduledTaskPrincipal -UserId 'Reechard' -LogonType Interactive -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
Register-ScheduledTask -TaskName 'winsched' -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Description 'WinSched cron scheduler'
