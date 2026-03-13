# radii5 installer — short URL entry point
# Usage: irm https://radii5.github.io/music/install.ps1 | iex
$url = "https://raw.githubusercontent.com/radii5/music/main/scripts/install.ps1"
$script = (New-Object System.Net.WebClient).DownloadString($url)
Invoke-Expression $script
