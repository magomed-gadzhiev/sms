$files = Get-ChildItem -Path "c:\projects\sms" -Recurse -Include "*.go" | Where-Object { $_.FullName -notmatch "node_modules|\.worktrees" }
$count = 0
foreach ($file in $files) {
    $content = Get-Content $file.FullName -Raw
    if ($content -match 'github\.com/smpp-server/smpp-server') {
        $newContent = $content -replace 'github\.com/smpp-server/smpp-server', 'github.com/magomed-gadzhiev/sms'
        Set-Content -Path $file.FullName -Value $newContent -NoNewline
        $count++
    }
}
Write-Host "Updated $count files"
