package model

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Session es el estado persistido del editor entre ejecuciones: el directorio
// raíz de la sesión, las pestañas abiertas (RUTAS, en orden) y la activa (como
// ruta y no como índice: una ruta sobrevive a pestañas que ya no existen, un
// índice se correría de lugar). Lo que no se persiste —cursor, viewport,
// undo/redo, foco, panel— es estado de sesión en vivo, no de archivo.
type Session struct {
	Version int      `json:"version"`
	Root    string   `json:"root"`
	Tabs    []string `json:"tabs"`
	Active  string   `json:"active"`
}

// LoadSession lee y deserializa la sesión de path. Una sesión inexistente
// devuelve la sesión cero con un error os.IsNotExist; un JSON inválido
// devuelve error sin pánico —la restauración del arranque nunca puede morir
// por una sesión vieja—.
func LoadSession(path string) (Session, error) {
	var s Session
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	return s, nil
}

// SaveSession escribe la sesión en path con indentación de dos espacios,
// creando el directorio anidado si falta (0o755) y escribiendo el archivo con
// permisos 0o600: la sesión puede contener rutas, no hay por qué dejarla leer
// a cualquiera.
func SaveSession(path string, s Session) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
