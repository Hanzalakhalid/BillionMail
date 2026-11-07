param([switch]$KeepServices,[switch]$Full)
$ErrorActionPreference = 'Stop'
$marketingRepo = Split-Path -Parent $PSScriptRoot
$marketingCertDirectory = Join-Path ([IO.Path]::GetTempPath()) ('billionmail-test-ca-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $marketingCertDirectory | Out-Null
$marketingPreviousCa = $env:BM_TEST_CA_DIRECTORY
$marketingPreviousFull = $env:BM_TEST_FULL
$marketingPreviousBuildCa = $env:BILLIONMAIL_BUILD_CA_FILE
try {
    # Public certificates only. Match the trust already configured on this host;
    # never disable TLS verification for module downloads.
    $marketingCertificates = Get-ChildItem Cert:\LocalMachine\Root,Cert:\CurrentUser\Root | Sort-Object Thumbprint -Unique
    $marketingPem = ($marketingCertificates | ForEach-Object {
        "-----BEGIN CERTIFICATE-----`n" + [Convert]::ToBase64String($_.RawData, [Base64FormattingOptions]::InsertLineBreaks) + "`n-----END CERTIFICATE-----`n"
    }) -join "`n"
    [IO.File]::WriteAllText((Join-Path $marketingCertDirectory 'ca-bundle.pem'), $marketingPem)
    $env:BM_TEST_CA_DIRECTORY = $marketingCertDirectory
    $env:BILLIONMAIL_BUILD_CA_FILE = Join-Path $marketingCertDirectory 'ca-bundle.pem'
    if ($Full) { $env:BM_TEST_FULL = '1' }
    Push-Location $marketingRepo
    try {
        # Each run receives a fresh database, including after a failed run.
        docker compose -f compose.test.yml down --remove-orphans
        if ($LASTEXITCODE -ne 0) { throw 'Could not clean the isolated test services' }
        docker compose -f compose.test.yml build minio minio-init
        if ($LASTEXITCODE -ne 0) { throw 'Could not build the isolated storage services' }
        docker compose -f compose.test.yml run --rm backend
        $marketingTestExitCode = $LASTEXITCODE
    } finally {
        if (-not $KeepServices) { docker compose -f compose.test.yml down --remove-orphans }
        Pop-Location
    }
    if ($marketingTestExitCode -ne 0) { throw "Backend checks failed (exit $marketingTestExitCode)" }
} finally {
    $env:BM_TEST_CA_DIRECTORY = $marketingPreviousCa
    $env:BM_TEST_FULL = $marketingPreviousFull
    $env:BILLIONMAIL_BUILD_CA_FILE = $marketingPreviousBuildCa
    # The resolved target is the exact unique directory created above.
    $marketingResolvedCertDirectory = (Resolve-Path -LiteralPath $marketingCertDirectory).Path
    $marketingResolvedTempDirectory = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    if (-not $marketingResolvedCertDirectory.StartsWith($marketingResolvedTempDirectory, [StringComparison]::OrdinalIgnoreCase)) { throw 'Unexpected certificate directory' }
    Remove-Item -LiteralPath $marketingResolvedCertDirectory -Recurse -Force
}
