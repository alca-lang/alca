// ULang++ 2.4 installer UI — strict black & white design.
// Built with: installer/build.ps1
using System;
using System.Drawing;
using System.Drawing.Imaging;
using System.IO;
using System.Reflection;
using System.Threading;
using System.Windows.Forms;

internal static class Program
{
    [STAThread]
    private static void Main()
    {
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
    private string _source;

    public SetupForm()
    {
        _source = AppDomain.CurrentDomain.BaseDirectory.TrimEnd(Path.DirectorySeparatorChar);

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
            Text = "u++ / upp — в консоли — это компилятор.\n\n" +
                   "  u++ build main.upp\n" +
                   "  u++ run main.upp\n" +
                   "  u++ version\n\n" +
                   "Откройте новое окно терминала, чтобы подхватить PATH."
        };
        _finishPanel.Controls.Add(_finishText);
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
        _status.Text = "DONE.";
    }

    private void DoInstall(string target)
    {
        try
        {
            SetStatus(5, "Creating directories…");
            Directory.CreateDirectory(target);
            Directory.CreateDirectory(Path.Combine(target, "std"));

            SetStatus(20, "Installing compiler…");
            Copy(Path.Combine(_source, "result", "ulang++.exe"), Path.Combine(target, "ulang++.exe"), true);
            Copy(Path.Combine(_source, "result", "ulang++.exe"), Path.Combine(target, "u++.exe"), true);
            Copy(Path.Combine(_source, "result", "libulang.dll"), Path.Combine(target, "libulang.dll"), true);

            SetStatus(45, "Installing runtime libraries…");
            string resultDir = Path.Combine(_source, "result");
            if (Directory.Exists(resultDir))
            {
                foreach (var f in Directory.GetFiles(resultDir, "*.dll"))
                    Copy(f, Path.Combine(target, Path.GetFileName(f)), true);
            }

            SetStatus(65, "Installing standard library…");
            string stdSource = Path.Combine(_source, "std");
            if (Directory.Exists(stdSource))
            {
                foreach (var f in Directory.GetFiles(stdSource, "*.upp", SearchOption.AllDirectories))
                {
                    string rel = f.Substring(stdSource.Length).TrimStart(Path.DirectorySeparatorChar);
                    string dst = Path.Combine(target, "std", rel);
                    Directory.CreateDirectory(Path.GetDirectoryName(dst));
                    Copy(f, dst, true);
                }
            }

            SetStatus(78, "Installing documentation…");
            foreach (var name in new[] { "LICENSE", "NOTICE", "README.md", "CHANGELOG.md" })
            {
                string src = Path.Combine(_source, name);
                if (File.Exists(src)) Copy(src, Path.Combine(target, name), true);
            }

            string ico = Path.Combine(target, "ULang++_FILE-ICO.ico");
            string icoSrc = Path.Combine(_source, "ULang++_FILE-ICO.ico");
            if (File.Exists(icoSrc)) Copy(icoSrc, ico, true);

            SetStatus(88, "Registering .upp association…");
            RegisterAssociation(target, ico);
            AddToUserPath(target);

            SetStatus(100, "Finishing…");
            ShowFinish();
        }
        catch (Exception ex)
        {
            SetStatus(0, "FAILED: " + ex.Message);
            BeginInvoke(new Action(() => MessageBox.Show(this, ex.Message, "ULang++ setup", MessageBoxButtons.OK, MessageBoxIcon.Error)));
        }
    }

    private static void Copy(string src, string dst, bool overwrite)
    {
        if (!File.Exists(src)) return;
        File.Copy(src, dst, overwrite);
    }

    private static void RegisterAssociation(string target, string ico)
    {
        using (var cls = Microsoft.Win32.Registry.CurrentUser.CreateSubKey(@"Software\Classes\.upp"))
        {
            cls.SetValue("", "ULang++.Source");
            using (var icon = cls.CreateSubKey("DefaultIcon"))
                icon.SetValue("", "\"" + ico + "\",0");
            using (var cmd = cls.CreateSubKey(@"shell\check\command"))
                cmd.SetValue("", "\"" + Path.Combine(target, "u++.exe") + "\" check \"%1\"");
        }
        using (var prog = Microsoft.Win32.Registry.CurrentUser.CreateSubKey(@"Software\Classes\ULang++.Source"))
            prog.SetValue("", "ULang++ source file");
    }

    private static void AddToUserPath(string target)
    {
        using (var env = Microsoft.Win32.Registry.CurrentUser.OpenSubKey("Environment", true))
        {
            if (env == null) return;
            string path = env.GetValue("Path", "") as string ?? "";
            if (path.IndexOf(target, StringComparison.OrdinalIgnoreCase) < 0)
                env.SetValue("Path", path.TrimEnd(';') + ";" + target, Microsoft.Win32.RegistryValueKind.ExpandString);
        }
    }
}
