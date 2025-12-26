# Requires the AudioDeviceCmdlets module to be installed.
# Install-Module -Name AudioDeviceCmdlets -Force -Scope CurrentUser (run this once as Admin if not installed)

Write-Host "Starting microphone volume monitoring..."

while ($true) {
    try {
        # Check if the module is loaded, and import it if not.
        # This helps ensure the cmdlets are available, especially when run by Task Scheduler.
        if (-not (Get-Module -ListAvailable -Name AudioDeviceCmdlets)) {
            Write-Warning "AudioDeviceCmdlets module not found. Please install it."
            # You might want to exit here or wait for module to be available if critical
            Start-Sleep -Seconds 60 # Wait longer if module is missing
            continue
        }
        if (-not (Get-Module -Name AudioDeviceCmdlets -ErrorAction SilentlyContinue)) {
            Import-Module AudioDeviceCmdlets -ErrorAction Stop
        }

        # Get the current volume of the default communication recording device
        # Note: Set-AudioDevice -RecordingVolume is NOT a valid parameter.
        # It should be -RecordingCommunicationVolume for the default communication device
        # OR -Volume with a specific device ID/object.
        # We will use -RecordingCommunicationVolume as per your previous successful command.

        $currentVolume = (Get-AudioDevice -List | Where-Object { $_.Type -eq "Recording" -and $_.IsDefaultCommunicationDevice }).Volume

        # If we couldn't get the volume for some reason, or it's not 100%
        if ($null -eq $currentVolume -or $currentVolume -ne 100) {
            Write-Host "Microphone volume is currently $($currentVolume -replace '^$', 'unknown')%. Setting to 100%..."
            # Try setting the volume using the communication device parameter set
            Set-AudioDevice -RecordingCommunicationVolume 100 -ErrorAction SilentlyContinue

            # Add a fallback for a specific device if the communication one is not working
            # (This is more complex for a simple loop, but good to keep in mind if needed)
            # Example fallback to find the first "Microphone" and set its volume:
            # $specificMic = Get-AudioDevice -List | Where-Object { $_.Type -eq "Recording" -and $_.Name -like "*Microphone*" } | Select-Object -First 1
            # if ($specificMic) {
            #    Set-AudioDevice -Id $specificMic.Id -Volume 100 -ErrorAction SilentlyContinue
            # }

        } else {
            # Write-Host "Microphone volume is already 100%. No action needed." # Uncomment for more verbose logging
        }

    }
    catch {
        Write-Error "An error occurred: $($_.Exception.Message)"
        # Log the error for debugging
    }

    # Wait for 10 seconds before the next loop
    Start-Sleep -Seconds 10
}
