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
func (sc *Scanner) ScanMusicFolder(ctx context.Context, path string) (*ScanResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("la carpeta no existe: %s", path)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("la ruta no es una carpeta: %s", path)
	}

	result := &ScanResult{}

	walkErr := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
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
		return result, fmt.Errorf("escaneando %s: %w", path, walkErr)
	}

	sc.log.Info("escaneo finalizado",
		"carpeta", path,
		"encontrados", result.FilesFound,
		"nuevos", result.Added,
		"existentes", result.Existing,
		"incompatibles", result.Incompatible,
		"errores", result.Errors,
	)
	return result, nil
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
