# answer_no.ps1 - Runs an Exchange command that may ask a question, and
# answers No. Dot-sourced by the scripts that need it, after auth.ps1.
#
# Some Exchange cmdlets ask a question that -Confirm:$false does not answer,
# and that -Force answers Yes. Enable-ExchangeCertificate -Services SMTP asks
# "Overwrite the existing default SMTP certificate?": Yes replaces the
# internal transport certificate of the server, which EdgeSync and the SMTP
# traffic between Exchange servers use. In the bridge's non-interactive
# PowerShell the question fails, after Exchange has written part of the change
# and generated a Watson dump on the server.
#
# The question goes to the host of the runspace that opened the remote
# session, not of the one that runs the command: Invoke-BridgeAnsweringNo
# opens a runspace with a host that answers No to every question, connects
# again from there with auth.ps1, runs the command and disconnects.

if (-not ("BridgeNoHost" -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Globalization;
using System.Management.Automation;
using System.Management.Automation.Host;
using System.Security;

// BridgeNoUI answers No to every choice, records the questions, and refuses
// any other input.
public class BridgeNoUI : PSHostUserInterface {
    public readonly List<string> Questions = new List<string>();
    public override PSHostRawUserInterface RawUI { get { return null; } }
    public override int PromptForChoice(string caption, string message, Collection<ChoiceDescription> choices, int defaultChoice) {
        Questions.Add(message);
        for (int i = 0; i < choices.Count; i++) {
            if (choices[i].Label.Replace("&", "").Equals("No", StringComparison.OrdinalIgnoreCase)) { return i; }
        }
        throw new HostException("Exchange asked a question without a No answer: " + message);
    }
    public override Dictionary<string, PSObject> Prompt(string caption, string message, Collection<FieldDescription> descriptions) {
        throw new HostException("Exchange asked for input: " + message);
    }
    public override PSCredential PromptForCredential(string caption, string message, string userName, string targetName) {
        throw new HostException("Exchange asked for a credential: " + message);
    }
    public override PSCredential PromptForCredential(string caption, string message, string userName, string targetName,
        PSCredentialTypes allowedCredentialTypes, PSCredentialUIOptions options) {
        throw new HostException("Exchange asked for a credential: " + message);
    }
    public override string ReadLine() { throw new HostException("Exchange asked for input"); }
    public override SecureString ReadLineAsSecureString() { throw new HostException("Exchange asked for input"); }
    public override void Write(string value) { }
    public override void Write(ConsoleColor foregroundColor, ConsoleColor backgroundColor, string value) { }
    public override void WriteLine(string value) { }
    public override void WriteErrorLine(string value) { }
    public override void WriteDebugLine(string message) { }
    public override void WriteProgress(long sourceId, ProgressRecord record) { }
    public override void WriteVerboseLine(string message) { }
    public override void WriteWarningLine(string message) { }
}

public class BridgeNoHost : PSHost {
    private readonly Guid id = Guid.NewGuid();
    public readonly BridgeNoUI NoUI = new BridgeNoUI();
    public override string Name { get { return "BridgeNoHost"; } }
    public override Version Version { get { return new Version(1, 0); } }
    public override Guid InstanceId { get { return id; } }
    public override PSHostUserInterface UI { get { return NoUI; } }
    public override CultureInfo CurrentCulture { get { return CultureInfo.GetCultureInfo("en-US"); } }
    public override CultureInfo CurrentUICulture { get { return CultureInfo.GetCultureInfo("en-US"); } }
    public override void SetShouldExit(int exitCode) { }
    public override void EnterNestedPrompt() { throw new NotSupportedException(); }
    public override void ExitNestedPrompt() { throw new NotSupportedException(); }
    public override void NotifyBeginApplication() { }
    public override void NotifyEndApplication() { }
}
'@
}

# Invoke-BridgeAnsweringNo runs $Command, a command line, with the Exchange
# cmdlets of a new connection, answering No to its questions. It returns the
# questions asked. $ConnectionServer and $ConnectionUser are the -Server and
# -Username of auth.ps1; the secrets are in the environment.
function Invoke-BridgeAnsweringNo([string]$Command, [string]$ConnectionServer, [string]$ConnectionUser) {
    $noHost = New-Object BridgeNoHost
    $runspace = [runspacefactory]::CreateRunspace($noHost)
    $runspace.Open()
    $ps = [powershell]::Create()
    $ps.Runspace = $runspace
    try {
        $run = {
            param($AuthScript, $AuthServer, $AuthUser, $Command)
            $ErrorActionPreference = "Stop"
            . $AuthScript -Server $AuthServer -Username $AuthUser -Password ""
            try { & ([scriptblock]::Create($Command)) | Out-Null } finally { Get-PSSession | Remove-PSSession }
        }
        [void]$ps.AddScript($run.ToString()).AddArgument("$PSScriptRoot\auth.ps1").AddArgument($ConnectionServer).AddArgument($ConnectionUser).AddArgument($Command)
        try {
            $ps.Invoke() | Out-Null
        } catch {
            $inner = $_.Exception
            while ($null -ne $inner.InnerException) { $inner = $inner.InnerException }
            throw $inner.Message
        }
        if ($ps.Streams.Error.Count -gt 0) { throw $ps.Streams.Error[0].ToString() }
        ,@($noHost.NoUI.Questions)
    } finally {
        $ps.Dispose()
        $runspace.Dispose()
    }
}
