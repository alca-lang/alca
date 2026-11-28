// ULang++ 2.4 installer UI — strict black & white design.
// Built with: installer/build.ps1
using System;
using System.Drawing;
using System.Drawing.Imaging;
using System.IO;
using System.IO.Compression;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Threading;
using System.Windows.Forms;

internal static class Program
{
    [STAThread]
    private static void Main()
    {
        foreach (string arg in Environment.GetCommandLineArgs())
        {
            if (arg.StartsWith("/DIR:", StringComparison.OrdinalIgnoreCase))
            {
                string dir = arg.Substring(5);
                string log = Path.Combine(Path.GetTempPath(), "ulang-setup.log");
                SetupForm.SilentInstall(dir, log);
                return;
            }
        }
        Application.EnableVisualStyles();
        Application.SetCompatibleTextRenderingDefault(false);
        Application.Run(new SetupForm());
    }
}

internal sealed class SetupForm : Form
{
    private const string Product = "ULANG++";
    private const string Version = "2.4";
    private const string DefaultDir = @"C:\Program Files\ULang++ 2.4";

    [DllImport("user32.dll", CharSet = CharSet.Auto)]
    private static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint msg, UIntPtr wParam,
        string lParam, uint flags, uint timeout, out UIntPtr result);
    [DllImport("shell32.dll")]
    private static extern void SHChangeNotify(int eventId, uint flags, IntPtr item1, IntPtr item2);
    [DllImport("user32.dll")]
    private static extern bool DestroyIcon(IntPtr handle);

    private static readonly Color Black = Color.FromArgb(8, 8, 8);
    private static readonly Color Panel = Color.FromArgb(18, 18, 18);
    private static readonly Color Line = Color.FromArgb(62, 62, 62);
    private static readonly Color Gray = Color.FromArgb(150, 150, 150);
    private static readonly Color White = Color.White;

    private readonly TextBox _pathBox = new TextBox();
    private readonly Button _browseButton = new Button();
    private readonly Button _installButton = new Button();
    private readonly ProgressBar _progress = new ProgressBar();
    private readonly Label _status = new Label();
    private readonly Panel _finishPanel = new Panel();
    private readonly Label _finishText = new Label();
    private readonly ProgressBar _finishProgress = new ProgressBar();

    public SetupForm()
    {
        Text = "ULang++ " + Version + " Setup";
        ClientSize = new Size(920, 600);
        FormBorderStyle = FormBorderStyle.FixedDialog;
        MaximizeBox = false;
        StartPosition = FormStartPosition.CenterScreen;
        BackColor = Black;
        Font = new Font("Segoe UI", 10F);

        // Left rail: logo + product mark.
        var rail = new Panel { Dock = DockStyle.Left, Width = 300, BackColor = Panel };
        var logo = new PictureBox { SizeMode = PictureBoxSizeMode.Zoom, Size = new Size(230, 190), Location = new Point(35, 42) };
        using (var stream = Assembly.GetExecutingAssembly().GetManifestResourceStream("ULangLogo"))
        {
            if (stream != null) logo.Image = ToGrayscale(Image.FromStream(stream));
        }
        rail.Controls.Add(logo);
        rail.Controls.Add(LabelText("ULANG++", 36, 264, 230, 44, 25F, White, true));
        rail.Controls.Add(LabelText("SETUP  " + Version, 38, 312, 230, 26, 12F, Gray));
        rail.Controls.Add(new Label { Location = new Point(36, 354), Size = new Size(228, 1), BackColor = Line });
        Controls.Add(rail);

        // Right content.
        int x = 332, w = 552;
        Controls.Add(LabelText("Install ULang++ " + Version, x, 72, w, 44, 26F, White, true));
        Controls.Add(new Label { Location = new Point(x, 132), Size = new Size(w, 1), BackColor = Line });

        Controls.Add(LabelText("DESTINATION", x, 184, w, 20, 9F, Gray));
        _pathBox.Location = new Point(x, 212);
        _pathBox.Size = new Size(w - 118, 36);
        _pathBox.Text = DefaultDir;
        _pathBox.BackColor = White;
        _pathBox.ForeColor = Black;
        _pathBox.BorderStyle = BorderStyle.FixedSingle;
        _pathBox.Font = new Font("Consolas", 10F);
        Controls.Add(_pathBox);

        _browseButton.Text = "BROWSE";
        _browseButton.Location = new Point(x + w - 108, 211);
        _browseButton.Size = new Size(108, 38);
        StyleOutline(_browseButton);
        _browseButton.Click += Browse_Click;
        Controls.Add(_browseButton);

        _status.Location = new Point(x, 276);
        _status.Size = new Size(w, 24);
        _status.AutoSize = false;
        _status.ForeColor = White;
        Controls.Add(_status);

        _progress.Location = new Point(x, 312);
        _progress.Size = new Size(w, 16);
        _progress.BackColor = Color.FromArgb(30, 30, 30);
        _progress.ForeColor = White;
        _progress.Style = ProgressBarStyle.Continuous;
        Controls.Add(_progress);

        _installButton.Text = "INSTALL  " + Product + "  " + Version;
        _installButton.Location = new Point(x, 372);
        _installButton.Size = new Size(w, 52);
        StyleSolid(_installButton);
        _installButton.Font = new Font("Segoe UI", 11F, FontStyle.Bold);
        _installButton.Click += Install_Click;
        Controls.Add(_installButton);

        // Finish view — black panel, white frame and text.
        _finishPanel.Location = new Point(x, 44);
        _finishPanel.Size = new Size(w, 470);
        _finishPanel.BackColor = Black;
        _finishPanel.BorderStyle = BorderStyle.FixedSingle;
        _finishPanel.Visible = false;
        _finishPanel.Controls.Add(LabelText("[OK]  INSTALLATION COMPLETE", 20, 18, w - 40, 34, 17F, White, true));
        _finishPanel.Controls.Add(new Label { Location = new Point(20, 58), Size = new Size(w - 40, 1), BackColor = Line });
        _finishText = new Label
        {
            Location = new Point(20, 78),
            Size = new Size(w - 40, 200),
            AutoSize = false,
            ForeColor = White,
            Font = new Font("Consolas", 11F),
            Text = "u++ / upp is the compiler. Type it in a console.\n\n" +
                   "  u++ build main.upp\n" +
                   "  u++ run main.upp\n" +
                   "  u++ version\n\n" +
                   "Open a new terminal to pick up PATH."
        };
        _finishPanel.Controls.Add(_finishText);

        _finishProgress.Location = new Point(20, 300);
        _finishProgress.Size = new Size(w - 40, 14);
        _finishProgress.BackColor = Color.FromArgb(30, 30, 30);
        _finishProgress.ForeColor = White;
        _finishProgress.Style = ProgressBarStyle.Continuous;
        _finishPanel.Controls.Add(_finishProgress);
        Controls.Add(_finishPanel);

        var close = new Button { Text = "CLOSE", Location = new Point(x + w - 108, 548), Size = new Size(108, 34) };
        StyleOutline(close);
        close.Click += (s, e) => Close();
        Controls.Add(close);
    }

    private static Image ToGrayscale(Image source)
    {
        var bmp = new Bitmap(source.Width, source.Height, PixelFormat.Format32bppArgb);
        using (var g = Graphics.FromImage(bmp))
        {
            var matrix = new ColorMatrix(new[]
            {
                new float[] { 0.299f, 0.299f, 0.299f, 0, 0 },
                new float[] { 0.587f, 0.587f, 0.587f, 0, 0 },
                new float[] { 0.114f, 0.114f, 0.114f, 0, 0 },
                new float[] { 0, 0, 0, 1, 0 },
                new float[] { 0, 0, 0, 0, 1 }
            });
            using (var attrs = new ImageAttributes())
            {
                attrs.SetColorMatrix(matrix);
                g.DrawImage(source, new Rectangle(0, 0, bmp.Width, bmp.Height),
                            0, 0, bmp.Width, bmp.Height, GraphicsUnit.Pixel, attrs);
            }
        }
        return bmp;
    }

    private static Label LabelText(string text, int x, int y, int w, int h, float size, Color color, bool bold = false)
    {
        return new Label
        {
            Text = text,
            Location = new Point(x, y),
            Size = new Size(w, h),
            AutoSize = false,
            ForeColor = color,
            Font = new Font("Segoe UI", size, bold ? FontStyle.Bold : FontStyle.Regular)
        };
    }

    private static void StyleOutline(Button b)
    {
        b.FlatStyle = FlatStyle.Flat;
        b.BackColor = Black;
        b.ForeColor = White;
        b.FlatAppearance.BorderColor = White;
        b.FlatAppearance.BorderSize = 1;
    }

    private static void StyleSolid(Button b)
    {
        b.FlatStyle = FlatStyle.Flat;
        b.BackColor = White;
        b.ForeColor = Black;
        b.FlatAppearance.BorderColor = White;
        b.FlatAppearance.BorderSize = 1;
    }

    private void Browse_Click(object sender, EventArgs e)
    {
        using (var dlg = new FolderBrowserDialog { SelectedPath = _pathBox.Text })
        {
            if (dlg.ShowDialog(this) == DialogResult.OK) _pathBox.Text = dlg.SelectedPath;
        }
    }

    private void Install_Click(object sender, EventArgs e)
    {
        string target = _pathBox.Text.Trim();
        if (target.Length == 0) { MessageBox.Show("Choose an installation folder."); return; }
        _installButton.Enabled = false;
        _browseButton.Enabled = false;
        _pathBox.Enabled = false;
        new Thread(() => DoInstall(target)).Start();
    }

    private void SetStatus(int percent, string text)
    {
        if (InvokeRequired) { BeginInvoke(new Action(() => SetStatus(percent, text))); return; }
        _progress.Value = Math.Max(0, Math.Min(100, percent));
        _status.Text = text;
    }

    private void ShowFinish()
    {
        if (InvokeRequired) { BeginInvoke(new Action(ShowFinish)); return; }
        _finishPanel.BringToFront();
        _finishPanel.Visible = true;
        _progress.Value = 100;
        _finishProgress.Value = 100;
        _status.Text = "DONE.";
    }

    private void DoInstall(string target)
    {
        try
        {
            PerformInstall(target, SetStatus);
            ShowFinish();
        }
        catch (Exception ex)
        {
            SetStatus(0, "FAILED: " + ex.Message);
            BeginInvoke(new Action(() => MessageBox.Show(this, ex.Message, "ULang++ setup", MessageBoxButtons.OK, MessageBoxIcon.Error)));
        }
    }

    internal static void SilentInstall(string target, string log)
    {
        try
        {
            PerformInstall(target, (p, s) => File.AppendAllText(log, p + "% " + s + Environment.NewLine));
            File.AppendAllText(log, "DONE" + Environment.NewLine);
        }
        catch (Exception ex)
        {
            File.AppendAllText(log, "FAILED: " + ex + Environment.NewLine);
        }
    }

    private static void PerformInstall(string target, Action<int, string> report)
    {
        report(5, "Creating directories…");
        Directory.CreateDirectory(target);

        report(20, "Unpacking compiler and standard library…");
        ExtractPayload(target);
        string ico = Path.Combine(target, "ULang++_FILE-ICO.ico");
        GenerateFileIcon(ico);

        report(88, "Registering .upp association…");
        RegisterAssociation(target, ico);
        AddToUserPath(target);

        report(100, "Finishing…");
    }

    private static void GenerateFileIcon(string path)
    {
        using (var stream = Assembly.GetExecutingAssembly().GetManifestResourceStream("FileIcon"))
        {
            if (stream == null) return;
            using (var img = Image.FromStream(stream))
            using (var bmp = new Bitmap(img, new Size(256, 256)))
            {
                IntPtr h = bmp.GetHicon();
                using (var icon = Icon.FromHandle(h))
                using (var fs = File.Create(path))
                {
                    icon.Save(fs);
                }
                DestroyIcon(h);
            }
        }
    }

    private static void ExtractPayload(string target)
    {
        using (var stream = Assembly.GetExecutingAssembly().GetManifestResourceStream("Payload"))
        {
            if (stream == null) throw new InvalidOperationException("Embedded payload is missing");
            using (var zip = new ZipArchive(stream, ZipArchiveMode.Read))
            {
                foreach (var entry in zip.Entries)
                {
                    string rel = entry.FullName.Replace('/', Path.DirectorySeparatorChar);
                    if (rel.Contains("..")) continue;
                    if (rel.EndsWith(Path.DirectorySeparatorChar.ToString())) continue;
                    string dst = Path.Combine(target, rel);
                    string dir = Path.GetDirectoryName(dst);
                    if (!string.IsNullOrEmpty(dir)) Directory.CreateDirectory(dir);
                    using (var input = entry.Open())
                    using (var output = File.Create(dst))
                    {
                        input.CopyTo(output);
                    }
                }
            }
        }
    }

    private static void Copy(string src, string dst, bool overwrite)
    {
        if (!File.Exists(src)) return;
        File.Copy(src, dst, overwrite);
    }

    private static void RegisterAssociation(string target, string ico)
    {
        string exe = Path.Combine(target, "u++.exe");
        foreach (var root in new[] { Microsoft.Win32.Registry.CurrentUser, Microsoft.Win32.Registry.LocalMachine })
        {
            try
            {
                using (var cls = root.CreateSubKey(@"Software\Classes\.upp"))
                {
                    cls.SetValue("", "ULang++.Source");
                    cls.SetValue("Content Type", "text/plain");
                    using (var icon = cls.CreateSubKey("DefaultIcon"))
                        icon.SetValue("", "\"" + ico + "\",0");
                    using (var cmd = cls.CreateSubKey(@"shell\open\command"))
                        cmd.SetValue("", "\"" + exe + "\" check \"%1\"");
                    using (var check = cls.CreateSubKey(@"shell\check\command"))
                        check.SetValue("", "\"" + exe + "\" check \"%1\"");
                }
                using (var prog = root.CreateSubKey(@"Software\Classes\ULang++.Source"))
                {
                    prog.SetValue("", "ULang++ source file");
                    using (var icon = prog.CreateSubKey("DefaultIcon"))
                        icon.SetValue("", "\"" + ico + "\",0");
                }
            }
            catch (UnauthorizedAccessException) { }
            catch (System.Security.SecurityException) { }
        }
        // Force Explorer to reload file associations.
        SHChangeNotify(0x08000000, 0x0000, IntPtr.Zero, IntPtr.Zero);
    }

    private static void AddToUserPath(string target)
    {
        bool changed = false;
        // Elevation can run under a different account, so HKCU is unreliable.
        // The machine PATH is visible to every user.
        try
        {
            string machine = Environment.GetEnvironmentVariable("Path", EnvironmentVariableTarget.Machine) ?? "";
            if (machine.IndexOf(target, StringComparison.OrdinalIgnoreCase) < 0)
            {
                Environment.SetEnvironmentVariable("Path", machine.TrimEnd(';') + ";" + target,
                                                    EnvironmentVariableTarget.Machine);
                changed = true;
            }
        }
        catch (UnauthorizedAccessException) { }
        catch (System.Security.SecurityException) { }

        try
        {
            string user = Environment.GetEnvironmentVariable("Path", EnvironmentVariableTarget.User) ?? "";
            if (user.IndexOf(target, StringComparison.OrdinalIgnoreCase) < 0)
            {
                Environment.SetEnvironmentVariable("Path", user.TrimEnd(';') + ";" + target,
                                                    EnvironmentVariableTarget.User);
                changed = true;
            }
        }
        catch (UnauthorizedAccessException) { }
        catch (System.Security.SecurityException) { }

        if (changed)
        {
            UIntPtr result;
            SendMessageTimeout(new IntPtr(0xffff), 0x001A, UIntPtr.Zero, "Path", 0x0002, 5000, out result);
        }
    }
}
