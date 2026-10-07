package ext

// GitInfo contiene información del estado de git del directorio del buffer
// activo. El host Lua recibe esta estructura como tabla desde tcode.git.status().
type GitInfo struct {
	StagedFiles    []string // Archivos en staging area (git add)
	UnstagedFiles  []string // Archivos modificados sin stagear
	UntrackedFiles []string // Archivos nuevos untracked
	AddedLines     int      // Total de líneas agregadas (staged + unstaged)
	DeletedLines   int      // Total de líneas borradas (staged + unstaged)
	Branch         string   // Rama actual (SHA corto si detached HEAD, "" si indeterminable)
	CommitHash     string   // SHA corto del último commit ("" si indeterminable)
	CommitSubject  string   // Subject del último commit ("" si indeterminable)
	CommitAuthor   string   // Autor del último commit ("" si indeterminable)
	CommitDate     string   // Fecha corta del último commit ("" si indeterminable)
}

// FileDiffLine representa una línea individual con cambios en un diff.
// Type puede ser "added", "deleted" o "modified".
type FileDiffLine struct {
	Line int    // Número de línea (1-indexado)
	Type string // Tipo de cambio: "added", "deleted", "modified"
}
