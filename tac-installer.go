package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	AppName       = "tac-writer"
	AppPrettyName = "TAC Writer"
	GithubUser    = "narayanls"

	AppInstallDir = "/usr/share/tac-writer"
	
	SuseDeps = "typelib-1_0-Gtk-4_0 typelib-1_0-Adw-1 libadwaita-1-0 python312-dropbox python313 python313-gobject python313-reportlab python313-pygtkspellcheck python313-pyenchant python313-Pillow python313-requests python313-pypdf python313-PyLaTeX gettext-runtime liberation-fonts myspell-pt_BR myspell-en_US myspell-es"
)

type DistroInfo struct {
	ID     string
	IDLike string
	Pretty string
}

type GithubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadUrl string `json:"browser_download_url"`
}

type GithubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt string        `json:"published_at"`
	Assets      []GithubAsset `json:"assets"`
}

func getVersionFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "tac-writer-version.txt")
	}
	return filepath.Join(home, ".local", "share", "tac-writer", "version.txt")
}

func writeInstalledVersion(version string) {
	_ = os.MkdirAll(AppInstallDir, 0755)
	vFile := getVersionFile()
	_ = os.MkdirAll(filepath.Dir(vFile), 0755)
	_ = os.WriteFile(vFile, []byte(version), 0644)
}

func getInstalledVersion() (string, error) {
	data, err := os.ReadFile(getVersionFile())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func compareVersions(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")

	max := len(as)
	if len(bs) > max {
		max = len(bs)
	}

	for i := 0; i < max; i++ {
		ai, bi := 0, 0
		if i < len(as) {
			fmt.Sscanf(as[i], "%d", &ai)
		}
		if i < len(bs) {
			fmt.Sscanf(bs[i], "%d", &bi)
		}

		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}

func checkIsInstalled() bool {
	path := filepath.Join(AppInstallDir, "main.py")
	_, err := os.Stat(path)
	return err == nil
}

func openApplication() {
	cmd := exec.Command("tac-writer")
	if err := cmd.Start(); err != nil {
		exec.Command("python3", filepath.Join(AppInstallDir, "main.py")).Start()
	}
}

func getLatestRelease(user, repo string) (*GithubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", user, repo)

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Go-Installer-Zenity")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub retornou erro %d", resp.StatusCode)
	}

	var release GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func findAssetUrl(release *GithubRelease, suffix string) (string, string, error) {
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, suffix) {
			if strings.Contains(asset.Name, "arm") || strings.Contains(asset.Name, "aarch64") {
				continue
			}
			return asset.Name, asset.BrowserDownloadUrl, nil
		}
	}
	return "", "", fmt.Errorf("nenhum arquivo %s encontrado", suffix)
}

func formatReleaseNotes(body string) string {
	body = strings.ReplaceAll(body, "&", "&amp;")
	body = strings.ReplaceAll(body, "<", "&lt;")
	body = strings.ReplaceAll(body, ">", "&gt;")
	body = strings.TrimSpace(body)

	if body == "" {
		return "Nenhuma descrição fornecida."
	}
	if len(body) > 1000 {
		body = body[:1000] + "\n\n... (ver mais no GitHub)"
	}
	return body
}

func formatDate(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	return t.Format("02/01/2006")
}

func main() {
	distro := getDistroInfo()
	ensureZenity(distro)

	if checkIsInstalled() {
		release, err := getLatestRelease(GithubUser, AppName)
		if err != nil {
			zenityError("Erro ao verificar atualizações:\n" + err.Error())
			os.Exit(1)
		}

		latest := strings.TrimPrefix(release.TagName, "v")
		installed, err := getInstalledVersion()

		needsUpdate := true
		if err == nil && compareVersions(installed, latest) >= 0 {
			needsUpdate = false
		}

		if needsUpdate {
			msg := fmt.Sprintf(
				"Atualização disponível\n\n<b>Versão instalada</b>: %s\n<b>Versão nova</b>: %s\n\nDeseja atualizar?",
				installed, latest,
			)
			if zenityQuestionCustomTitle(msg, "Atualizar o "+AppPrettyName) {
				goto INSTALL_FLOW
			}
			os.Exit(0)
		}

		if zenityQuestionCustomTitle(
			"O <b>"+AppPrettyName+"</b> já está instalado e atualizado.\n\nDeseja abrir?",
			"Abrir",
		) {
			openApplication()
		}
		os.Exit(0)
	}

INSTALL_FLOW:

	release, err := getLatestRelease(GithubUser, AppName)
	if err != nil {
		zenityError("Erro ao consultar GitHub:\n" + err.Error())
		os.Exit(1)
	}

	version := strings.TrimPrefix(release.TagName, "v")
	date := formatDate(release.PublishedAt)
	news := formatReleaseNotes(release.Body)

	msg := fmt.Sprintf(
		"<b>%s</b> será instalado no seu computador.\n\n<b>Versão</b>: %s\n<b>Lançamento</b>: %s\n<b>Sistema</b>: %s\n\n<b>Novidades:</b>\n<span size='small'>%s</span>\n\nDeseja continuar?",
		AppPrettyName, version, date, distro.Pretty, news,
	)

	if !zenityQuestion(msg) {
		os.Exit(0)
	}

	var suffix, installCmd string

	switch {
	case strings.Contains(distro.ID, "arch") || strings.Contains(distro.IDLike, "arch") || strings.Contains(distro.IDLike, "manjaro") || strings.Contains(distro.ID, "manjaro") || strings.Contains(distro.ID, "cachyos"):
		suffix = ".pkg.tar.zst"
		installCmd = "pacman -U --noconfirm"

	case strings.Contains(distro.ID, "debian") || strings.Contains(distro.IDLike, "debian") || strings.Contains(distro.ID, "ubuntu"):
		suffix = ".deb"
		installCmd = "apt install -y"

	case strings.Contains(distro.ID, "fedora") || strings.Contains(distro.IDLike, "fedora") || strings.Contains(distro.IDLike, "bazzite") || strings.Contains(distro.ID, "bazzite") ||
		strings.Contains(distro.ID, "suse") || strings.Contains(distro.IDLike, "suse"):
		suffix = ".rpm"
		installCmd = "dnf install -y"

		if strings.Contains(distro.ID, "suse") || strings.Contains(distro.IDLike, "suse") {
			cmdDeps := fmt.Sprintf("pkexec zypper --non-interactive install -y %s", SuseDeps)
			errDeps := exec.Command("bash", "-c", cmdDeps).Run()
			if errDeps != nil {
				fmt.Println("Aviso: Falha ao instalar dependências do SUSE ou cancelado pelo usuário.")
			}
			
			installCmd = "zypper --non-interactive install -y --allow-unsigned-rpm"
		}

	default:
		zenityError("Distribuição não suportada")
		os.Exit(1)
	}

	fileName, url, err := findAssetUrl(release, suffix)
	if err != nil {
		zenityError(err.Error())
		os.Exit(1)
	}

	tmp := filepath.Join(os.TempDir(), fileName)
	if err := downloadFile(url, tmp); err != nil {
		zenityError("Erro no download:\n" + err.Error())
		os.Exit(1)
	}

	if installPackage(installCmd, tmp) {
		writeInstalledVersion(version)
		if zenityQuestionCustomTitle("Instalação concluída!\nDeseja abrir agora?", "Sucesso") {
			openApplication()
		}
	} else {
		zenityError("Falha na instalação ou a operação foi cancelada.")
	}

	os.Remove(tmp)
}

func getDistroInfo() DistroInfo {
	file, _ := os.Open("/etc/os-release")
	defer file.Close()

	info := DistroInfo{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			info.ID = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		} else if strings.HasPrefix(line, "ID_LIKE=") {
			info.IDLike = strings.Trim(strings.TrimPrefix(line, "ID_LIKE="), "\"")
		} else if strings.HasPrefix(line, "PRETTY_NAME=") {
			info.Pretty = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}
	info.ID = strings.ToLower(info.ID)
	info.IDLike = strings.ToLower(info.IDLike)
	return info
}

func ensureZenity(d DistroInfo) {
	if _, err := exec.LookPath("zenity"); err == nil {
		return
	}
	var cmd string
	if strings.Contains(d.ID, "arch") {
		cmd = "pacman -S --noconfirm zenity"
	} else if strings.Contains(d.ID, "debian") {
		cmd = "apt-get update && apt-get install -y zenity"
	} else if strings.Contains(d.ID, "fedora") {
		cmd = "dnf install -y zenity"
	} else if strings.Contains(d.ID, "suse") {
		cmd = "zypper --non-interactive install -y zenity"
	}
	if cmd != "" {
		exec.Command("pkexec", "bash", "-c", cmd).Run()
	}
}

func downloadFile(url, path string) error {
	cmd := fmt.Sprintf(
		"wget -O '%s' '%s' 2>&1 | zenity --progress --pulsate --title='Baixando...' --auto-close",
		path, url,
	)
	return exec.Command("bash", "-c", cmd).Run()
}

func installPackage(cmd, file string) bool {
	c := fmt.Sprintf("pkexec %s '%s'", cmd, file)
	return exec.Command("bash", "-c", c).Run() == nil
}

func zenityQuestion(text string) bool {
	return zenityQuestionCustomTitle(text, "Instalador do "+AppPrettyName)
}

func zenityQuestionCustomTitle(text, title string) bool {
	return exec.Command("zenity", "--question", "--title="+title, "--text="+text, "--width=500").Run() == nil
}

func zenityError(text string) {
	exec.Command("zenity", "--error", "--text="+text, "--width=400").Run()
}
