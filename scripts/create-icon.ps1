Add-Type -AssemblyName System.Drawing

# Use short path to avoid encoding issues
$shortPath = [System.IO.Path]::GetFullPath("D:\软件\得到GUI\build\appicon.png")
$source = "D:\软件\得到GUI\build\appicon.png"
$target = "D:\软件\得到GUI\build\windows\icon.ico"

Write-Host "Source: $source"
Write-Host "Target: $target"

$sizes = @(16, 32, 48, 64, 128, 256)

$img = [System.Drawing.Image]::FromFile($source)
Write-Host "Image: $($img.Width)x$($img.Height)"
Write-Host "Creating icon..."

$headerSize = 6 + ($sizes.Count * 16)
$fs = New-Object System.IO.FileStream($target, [System.IO.FileMode]::Create)
$bw = New-Object System.IO.BinaryWriter($fs)

# ICONDIR header
$bw.Write([byte]0)
$bw.Write([byte]0)
$bw.Write([byte]1)
$bw.Write([byte]0)
$bw.Write([byte]($sizes.Count))
$bw.Write([byte]0)

$dataOffset = $headerSize
$allData = @()

foreach ($s in $sizes) {
    $bmp = New-Object System.Drawing.Bitmap($s, $s)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.Clear([System.Drawing.Color]::Transparent)
    $g.DrawImage($img, 0, 0, $s, $s)
    $g.Dispose()
    
    $ms = New-Object System.IO.MemoryStream
    $bmp.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
    $bmp.Dispose()
    
    $data = $ms.ToArray()
    $ms.Close()
    
    $allData += ,@{Size=$s; Data=$data}
    
    # Directory entry
    if ($s -eq 256) { $bw.Write([byte]0) } else { $bw.Write([byte]$s) }
    if ($s -eq 256) { $bw.Write([byte]0) } else { $bw.Write([byte]$s) }
    $bw.Write([byte]0)
    $bw.Write([byte]0)
    $bw.Write([byte]0)
    $bw.Write([byte]0)
    $bw.Write([byte]32)
    $bw.Write([byte]0)
    $len = [uint32]$data.Length
    $bw.Write([byte]($len -band 0xFF))
    $bw.Write([byte](($len -shr 8) -band 0xFF))
    $bw.Write([byte](($len -shr 16) -band 0xFF))
    $bw.Write([byte](($len -shr 24) -band 0xFF))
    $bw.Write([byte]($dataOffset -band 0xFF))
    $bw.Write([byte](($dataOffset -shr 8) -band 0xFF))
    $bw.Write([byte](($dataOffset -shr 16) -band 0xFF))
    $bw.Write([byte](($dataOffset -shr 24) -band 0xFF))
    
    $dataOffset += $data.Length
}

foreach ($item in $allData) {
    $bw.Write($item.Data)
}

$bw.Close()
$fs.Close()
$img.Dispose()

Write-Host "Done! Size:" ([math]::Round((Get-Item $target).Length / 1KB, 2)) "KB"
Write-Host "Sizes:" ($sizes -join ', ')
