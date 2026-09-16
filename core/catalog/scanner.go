package catalog

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// supportedExtensions define los formatos de audio aceptados por el scanner.
var supportedExtensions = map[string]bool{
	".mp3":  true,
	".wav":  true,
	".flac": true,
	".ogg":  true,
	".m4a":  true,
	".wma":  true,
}

// ScanResult resume el resultado de un escaneo de carpeta.
type ScanResult struct {
	FilesFound   int `json:"files_found"`  // total de archivos encontrados
	Added        int `json:"added"`        // canciones nuevas registradas
	Existing     int `json:"existing"`     // ya estaban en el catálogo
	Incompatible int `json:"incompatible"` // extensión no soportada
	Errors       int `json:"errors"`       // archivos que no pudieron registrarse
}

// Scanner recorre carpetas de música y registra las canciones en el catálogo.
type Scanner struct {
	repo Repository
	log  *slog.Logger
}

// NewScanner crea un scanner de carpetas de música.
func NewScanner(repo Repository, log *slog.Logger) *Scanner {
	return &Scanner{repo: repo, log: log}
}

// ScanMusicFolder recorre recursivamente `path`, detecta archivos de audio
// compatibles y los registra en SQLite guardando su ruta absoluta.
// Evita duplicados usando file_path como clave única.
//
// Mitigación CWE-22 / go/path-injection: valida y canonaliza el path
// antes de cualquier acceso al filesystem para evitar path traversal.
func (sc *Scanner) ScanMusicFolder(ctx context.Context, path string) (*ScanResult, error) {
	safePath, err := sc.validateScanPath(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(safePath)
	if err != nil {
		return nil, fmt.Errorf("la carpeta no existe: %s", safePath)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("la ruta no es una carpeta: %s", safePath)
	}

	result := &ScanResult{}

	walkErr := filepath.WalkDir(safePath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			sc.log.Warn("no se pudo acceder", "path", p, "error", err)
			return nil // continuar con el resto
		}
		if d.IsDir() {
			return nil
		}
		// Respetar cancelación.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		result.FilesFound++

		ext := strings.ToLower(filepath.Ext(p))
		if !supportedExtensions[ext] {
			result.Incompatible++
			return nil
		}

		absPath, err := filepath.Abs(p)
		if err != nil {
			sc.log.Warn("no se pudo resolver ruta absoluta", "path", p, "error", err)
			result.Errors++
			return nil
		}
		// Defensa en profundidad: evitar que symlinks dentro de la carpeta
		// escapen del directorio raíz escaneado (path traversal vía symlink).
		if !isWithinRoot(safePath, absPath) {
			sc.log.Warn("ruta fuera del directorio raíz, se omite", "path", absPath, "root", safePath)
			result.Errors++
			return nil
		}

		exists, err := sc.repo.ExistsByPath(ctx, absPath)
		if err != nil {
			return fmt.Errorf("verificando duplicado %s: %w", absPath, err)
		}
		if exists {
			result.Existing++
			return nil
		}

		song := songFromFile(absPath)
		if err := sc.repo.Insert(ctx, song); err != nil {
			sc.log.Warn("no se pudo registrar", "path", absPath, "error", err)
			result.Errors++
			return nil
		}
		result.Added++
		return nil
	})
	if walkErr != nil {
		return result, fmt.Errorf("escaneando %s: %w", safePath, walkErr)
	}

	sc.log.Info("escaneo finalizado",
		"carpeta", safePath,
		"encontrados", result.FilesFound,
		"nuevos", result.Added,
		"existentes", result.Existing,
		"incompatibles", result.Incompatible,
		"errores", result.Errors,
	)
	return result, nil
}

// validateScanPath canonaliza y valida el path solicitado para el escaneo.
// Previene CWE-22 / go/path-injection:
//
//   - Rechaza entradas vacías y con byte nulo.
//   - Usa filepath.Clean + filepath.Abs + filepath.EvalSymlinks para obtener
//     la ruta canónica y resolver "..", "." y symlinks.
//   - Restringe el escaneo a un directorio raíz seguro (ROCKOLA_MUSIC_ROOT).
//   - Retorna la ruta absoluta y evaluada lista para usar con os.Stat/WalkDir.
func (sc *Scanner) validateScanPath(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("la ruta no puede estar vacía")
	}
	if strings.Contains(trimmed, "\x00") {
		return "", fmt.Errorf("ruta inválida: contiene carácter nulo")
	}

	// filepath.Clean normaliza separadores y elimina "." y ".." redundantes.
	cleaned := filepath.Clean(trimmed)

	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("ruta inválida: %w", err)
	}
	abs = filepath.Clean(abs)

	// Resolver symlinks para evitar bypass (ej: /music/link -> /etc).
	// Si el path no existe, EvalSymlinks falla con NotExist; en ese caso
	// devolvemos abs para que el posterior os.Stat genere el error esperado.
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if os.IsNotExist(err) {
			real = abs
		} else {
			return "", fmt.Errorf("ruta inválida: %w", err)
		}
	}

	// Validación final de forma.
	if !filepath.IsAbs(real) {
		return "", fmt.Errorf("ruta inválida: debe ser absoluta: %s", input)
	}
	if real == "" {
		return "", fmt.Errorf("ruta inválida: vacía tras normalizar")
	}

	// Restringir escaneo a raíz permitida.
	allowedRoot := strings.TrimSpace(os.Getenv("ROCKOLA_MUSIC_ROOT"))
	if allowedRoot == "" {
		allowedRoot = "."
	}
	allowedAbs, err := filepath.Abs(filepath.Clean(allowedRoot))
	if err != nil {
		return "", fmt.Errorf("configuración inválida de ROCKOLA_MUSIC_ROOT: %w", err)
	}
	allowedReal, err := filepath.EvalSymlinks(allowedAbs)
	if err != nil {
		if os.IsNotExist(err) {
			allowedReal = allowedAbs
		} else {
			return "", fmt.Errorf("configuración inválida de ROCKOLA_MUSIC_ROOT: %w", err)
		}
	}

	if !isWithinRoot(allowedReal, real) {
		return "", fmt.Errorf("ruta fuera del directorio permitido")
	}

	return real, nil
}

// isWithinRoot verifica que target esté dentro de root (o sea el mismo root).
// Usa filepath.Rel y rechaza cualquier target cuyo relativo comience con "..".
// Maneja volúmenes en Windows (ej: C:\ vs D:\ -> fuera).
func isWithinRoot(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)

	// En Windows, volúmenes distintos nunca están contenidos.
	if volRoot := filepath.VolumeName(root); volRoot != "" {
		if volTarget := filepath.VolumeName(target); !strings.EqualFold(volRoot, volTarget) {
			return false
		}
	}

	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	// ".." o "../algo" indica escape del root.
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	return true
}

// songFromFile construye una Song a partir de la ruta del archivo.
// MVP: sin lectura de metadata; el título se deriva del nombre del archivo.
// Si el nombre sigue el patrón "Artista - Título", se separan ambos campos.
func songFromFile(absPath string) *Song {
	name := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))

	title := name
	artist := ""
	if parts := strings.SplitN(name, " - ", 2); len(parts) == 2 {
		artist = strings.TrimSpace(parts[0])
		title = strings.TrimSpace(parts[1])
	}

	now := time.Now().UTC()
	return &Song{
		Title:     title,
		Artist:    artist,
		FilePath:  absPath,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
