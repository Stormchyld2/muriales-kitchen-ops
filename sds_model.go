package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	appID    = "muriales-kitchen-ops"
	basePort = 47715
	maxPDF   = 150 << 20
)

type Product struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Manufacturer       string `json:"manufacturer"`
	Supplier           string `json:"supplier"` // who it is bought from, e.g. Sysco
	Tags               string `json:"tags"`     // optional extra uses, comma separated
	Aliases            string `json:"aliases"`  // other names it is known by, e.g. the supplier's name for it
	SHA256             string `json:"sha256"`   // fingerprint of the PDF as it was added
	Category           string `json:"category"`
	Location           string `json:"location"`
	RevisionDate       string `json:"revisionDate"`
	Hazards            string `json:"hazards"`
	FirstAidEyes       string `json:"firstAidEyes"`
	FirstAidSkin       string `json:"firstAidSkin"`
	FirstAidInhalation string `json:"firstAidInhalation"`
	FirstAidSwallowed  string `json:"firstAidSwallowed"`
	PPE                string `json:"ppe"`
	// The GHS hazard pictograms and signal word shown in Section 2 of the
	// sheet, as ticked by a manager. They are part of the emergency summary.
	Pictograms   []string `json:"pictograms"`
	SignalWord   string   `json:"signalWord"`
	Pages        int      `json:"pages"`
	File         string   `json:"file"`    // file name inside the sheets folder
	Missing      bool     `json:"missing"` // set when the PDF is no longer in the folder
	PDFUpdatedAt string   `json:"pdfUpdatedAt"`
	// NeedsReview marks a product made from a PDF copied into the folder
	// by hand, until a manager completes its details.
	NeedsReview bool `json:"needsReview"`
	// Who checked the emergency summary against the manufacturer's sheet,
	// when, and which revision of the sheet it was checked against.
	SummaryVerifiedAt     string `json:"summaryVerifiedAt"`
	SummaryVerifiedBy     string `json:"summaryVerifiedBy"`
	SummarySourceRevision string `json:"summarySourceRevision"`
	// SummaryNeedsCheck is set when the PDF is replaced after the summary
	// was verified, so old first-aid text is never trusted against a new sheet.
	SummaryNeedsCheck bool `json:"summaryNeedsCheck"`
	// Set only on removed products, which are kept so they can be restored.
	RemovedAt    string `json:"removedAt,omitempty"`
	ArchivedFile string `json:"archivedFile,omitempty"`
	AddedAt      string `json:"addedAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// BinderItem is what the paper binder held for one product when it was last
// recorded as up to date.
type BinderItem struct {
	Name         string `json:"name"`
	Manufacturer string `json:"manufacturer"`
	Category     string `json:"category"`
	Location     string `json:"location"`
	RevisionDate string `json:"revisionDate"`
	PDFUpdatedAt string `json:"pdfUpdatedAt"`
	SHA256       string `json:"sha256"`
}

// Binder is what the paper binder holds, as confirmed by a manager after
// printing and putting the pages in. UpdatedAt is that confirmation.
type Binder struct {
	UpdatedAt   string                `json:"updatedAt"`
	GeneratedAt string                `json:"generatedAt"`
	Pages       int                   `json:"pages"`
	Items       map[string]BinderItem `json:"items"`
}

// BuildRecord is the last book the program assembled. Assembling a book is
// not proof that it was printed, so it is kept apart from Binder until a
// manager confirms.
type BuildRecord struct {
	At          string                `json:"at"`
	Scope       string                `json:"scope"` // all, selected or index
	Pages       int                   `json:"pages"`
	Complete    bool                  `json:"complete"`
	Missing     []string              `json:"missing"`
	Items       map[string]BinderItem `json:"items"`
	Confirmed   bool                  `json:"confirmed"`
	ConfirmedAt string                `json:"confirmedAt"`
}

type Library struct {
	Categories []string     `json:"categories"`
	Locations  []string     `json:"locations"`
	Products   []*Product   `json:"products"`
	Removed    []*Product   `json:"removed"`
	Notes      []*Note      `json:"notes"`
	Binder     *Binder      `json:"binder"`
	LastBuild  *BuildRecord `json:"lastBuild"`
	Schema     int          `json:"schema"`
	// Starter sheets already copied in, so one that a manager removes
	// does not come back the next time the program opens.
	StarterApplied []string `json:"starterApplied"`
}

// catchAll is the category that always exists, so no product is ever left
// without one.
const catchAll = "Other / Miscellaneous"

var defaultCategories = []string{
	"Dish Machine", "Sanitizers & Disinfectants", "Degreasers", "General Cleaning",
	"Floor Care", "Restroom Cleaning", "Laundry", "Drain Maintenance",
	"Descalers & Delimers", "Pest Control", "Hand Hygiene", "Maintenance Chemicals",
	"Air Care & Deodorizers", catchAll,
}

var defaultLocations = []string{
	"Drystock", "Mop Room", "Dish Area", "Prep Area", "Landing Area", "Barn",
}

// The list shipped before version 1.5.2. A library still holding exactly this list, which no
// manager changed, is moved to the current list; a customised list is left alone.
var oldDefaultLocations = []string{"Dish Room", "Janitorial Closet", "Kitchen Chemical Storage", "Laundry Area", "Maintenance Room"}

func sameList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]bool{}
	for _, x := range a {
		m[x] = true
	}
	for _, x := range b {
		if !m[x] {
			return false
		}
	}
	return true
}

func (s *store) migrateLocations() bool {
	if sameList(s.lib.Locations, oldDefaultLocations) {
		s.lib.Locations = append([]string{}, defaultLocations...)
		return true
	}
	return false
}

// upgrade brings a library made by an earlier version up to the current
// category list: the three renamed categories carry their products across,
// and the new categories are added without disturbing any the manager made.
func (s *store) upgrade() bool {
	if s.lib.Schema >= 3 {
		return false
	}
	for _, p := range s.lib.Products {
		if p.PDFUpdatedAt == "" {
			p.PDFUpdatedAt = p.UpdatedAt
		}
	}
	if s.lib.Schema == 2 {
		s.lib.Schema = 3
		return true
	}
	renamed := map[string]string{"Sanitizers": "Sanitizers & Disinfectants", "Restroom": "Restroom Cleaning", "Other": catchAll}
	have := map[string]bool{}
	var cats []string
	for _, c := range s.lib.Categories {
		if to, ok := renamed[c]; ok {
			c = to
		}
		if c != catchAll && !have[c] {
			have[c] = true
			cats = append(cats, c)
		}
	}
	for _, c := range defaultCategories {
		if c != catchAll && !have[c] {
			cats = append(cats, c)
		}
	}
	s.lib.Categories = append(cats, catchAll)
	for _, p := range s.lib.Products {
		if to, ok := renamed[p.Category]; ok {
			p.Category = to
		}
	}
	if len(s.lib.Locations) == 0 {
		s.lib.Locations = defaultLocations
	}
	s.lib.Schema = 3
	return true
}

type store struct {
	mu  sync.Mutex
	dir string // the library folder
	cfg string // settings file that remembers a library folder chosen by a manager
	lib Library
	hc  map[string]hashEntry // fingerprints already worked out, by file path
	lk  lockState
}

type settings struct {
	DataFolder string `json:"dataFolder"`
}

// relocate points the program at a different library folder, such as a
// OneDrive or network folder. If that folder already holds a library it is
// opened as it is; otherwise the current library is copied there. The old
// folder is never deleted. It reports whether the library was copied.
func (s *store) relocate(target string) (bool, error) {
	target = filepath.Clean(strings.TrimSpace(strings.Trim(strings.TrimSpace(target), `"`)))
	if !filepath.IsAbs(target) {
		return false, errors.New("enter the full path of the folder, for example C:\\Users\\Name\\OneDrive\\SDS Data")
	}
	if strings.EqualFold(target, filepath.Clean(s.dir)) {
		return false, errors.New("the library is already in that folder")
	}
	if rel, err := filepath.Rel(s.dir, target); err == nil && !strings.HasPrefix(rel, "..") {
		return false, errors.New("choose a folder that is not inside the current library folder")
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return false, fmt.Errorf("that folder could not be opened or created: %v", err)
	}
	probe := filepath.Join(target, ".sds-write-test")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return false, fmt.Errorf("the program is not allowed to save files in that folder: %v", err)
	}
	os.Remove(probe)

	// Only one person may edit a library at a time, wherever it lives.
	if cur, ok := s.claim(target, false); !ok {
		who := "someone"
		if cur != nil {
			who = cur.who()
		}
		return false, fmt.Errorf("%s is editing the library in that folder. Ask them to close the program first", who)
	}
	committed := false
	defer func() {
		if !committed {
			s.unlock(target)
		}
	}()

	parts := []string{"library.json", "sheets", "removed"}
	copied := false
	cleanup := func() {
		if copied {
			for _, n := range parts {
				_ = os.RemoveAll(filepath.Join(target, n))
			}
		}
	}
	if !exists(filepath.Join(target, "library.json")) {
		for _, n := range parts[1:] {
			if exists(filepath.Join(target, n)) {
				return false, fmt.Errorf("that folder already has a %q folder but no library. Choose an empty folder", n)
			}
		}
		copied = true
		err := copyFile(s.file(), filepath.Join(target, "library.json"))
		for _, n := range parts[1:] {
			if err != nil {
				break
			}
			if err = os.MkdirAll(filepath.Join(target, n), 0o755); err != nil {
				break
			}
			entries, _ := os.ReadDir(filepath.Join(s.dir, n))
			for _, e := range entries {
				if e.IsDir() || strings.HasSuffix(e.Name(), ".tmp") {
					continue
				}
				if err = copyFile(filepath.Join(s.dir, n, e.Name()), filepath.Join(target, n, e.Name())); err != nil {
					break
				}
			}
		}
		if err != nil {
			cleanup()
			return false, fmt.Errorf("the library could not be copied, so nothing was changed: %v", err)
		}
	}
	// The maintenance files live beside the library folder; bring them along.
	if oldM, newM := s.maintDir(), filepath.Join(filepath.Dir(target), "Maintenance Data"); !strings.EqualFold(oldM, newM) && !exists(filepath.Join(newM, "equipment.json")) {
		if entries, err := os.ReadDir(oldM); err == nil {
			_ = os.MkdirAll(newM, 0o755)
			for _, e := range entries {
				if !e.IsDir() && !strings.HasSuffix(e.Name(), ".tmp") {
					_ = copyFile(filepath.Join(oldM, e.Name()), filepath.Join(newM, e.Name()))
				}
			}
		}
	}
	// Open the library in its new home before committing to it.
	next := &store{dir: target}
	if err := next.load(); err != nil {
		cleanup()
		return false, fmt.Errorf("the library in that folder did not open correctly, so nothing was changed: %v", err)
	}
	if s.cfg != "" {
		b, _ := json.MarshalIndent(settings{DataFolder: target}, "", "  ")
		if err := os.WriteFile(s.cfg, b, 0o644); err != nil {
			cleanup()
			return false, fmt.Errorf("the new location could not be remembered, so nothing was changed: %v", err)
		}
	}
	committed = true
	s.unlock(s.dir)
	s.lk.readOnly.Store(false)
	s.dir, s.lib = target, next.lib
	return copied, nil
}

func (s *store) file() string { return filepath.Join(s.dir, "library.json") }

func (s *store) pdfDir() string { return filepath.Join(s.dir, "sheets") }

func (s *store) goneDir() string { return filepath.Join(s.dir, "removed") }

func (s *store) pdfPath(p *Product) string {
	if p.File == "" { // libraries made by version 1.0 named files by id
		return filepath.Join(s.pdfDir(), p.ID+".pdf")
	}
	return filepath.Join(s.pdfDir(), p.File)
}

var badFileChars = regexp.MustCompile(`[^A-Za-z0-9 &()_.,'+-]+`)

// fileNameFor picks a readable, unused file name in the sheets folder, so the
// folder makes sense to a person browsing it in Windows.
func (s *store) fileNameFor(name string) string {
	base := strings.Join(strings.Fields(badFileChars.ReplaceAllString(name, " ")), " ")
	base = strings.Trim(base, " .")
	if len(base) > 80 {
		base = strings.TrimSpace(base[:80])
	}
	if base == "" {
		base = "SDS"
	}
	for i := 1; ; i++ {
		candidate := base + ".pdf"
		if i > 1 {
			candidate = fmt.Sprintf("%s (%d).pdf", base, i)
		}
		if _, err := os.Stat(filepath.Join(s.pdfDir(), candidate)); errors.Is(err, fs.ErrNotExist) {
			return candidate
		}
	}
}

// applyStarter copies in any sheet that ships with the program and has not
// been offered to this library before.
func (s *store) applyStarter() bool {
	b, err := starterFiles.ReadFile("starter/library.json")
	if err != nil {
		return false
	}
	var st Library
	if err := json.Unmarshal(b, &st); err != nil {
		log.Printf("starter library could not be read: %v", err)
		return false
	}
	applied := map[string]bool{}
	for _, id := range s.lib.StarterApplied {
		applied[id] = true
	}
	changed := false
	for _, p := range st.Products {
		if applied[p.ID] {
			continue
		}
		s.lib.StarterApplied = append(s.lib.StarterApplied, p.ID)
		changed = true
		if s.find(p.ID) != nil {
			continue
		}
		pdf, err := starterFiles.ReadFile("starter/sheets/" + p.File)
		if err != nil {
			log.Printf("starter sheet %q is missing from the program: %v", p.File, err)
			continue
		}
		p.File = s.fileNameFor(strings.TrimSuffix(p.File, ".pdf"))
		if err := os.WriteFile(s.pdfPath(p), pdf, 0o644); err != nil {
			log.Printf("starter sheet %q could not be copied: %v", p.File, err)
			continue
		}
		p.AddedAt, p.UpdatedAt, p.PDFUpdatedAt, p.SHA256 = now(), now(), now(), sumBytes(pdf)
		s.lib.Products = append(s.lib.Products, p)
	}
	return changed
}

// scan keeps the library in step with the sheets folder: a PDF that someone
// copied into the folder by hand becomes a product, and a product whose PDF
// has gone is flagged. It reports whether the library changed.
func (s *store) scan() bool {
	known := map[string]bool{}
	baselined := false
	for _, p := range s.lib.Products {
		known[strings.ToLower(filepath.Base(s.pdfPath(p)))] = true
		_, err := os.Stat(s.pdfPath(p))
		p.Missing = err != nil
		if err == nil && p.SHA256 == "" {
			// A library from before fingerprints existed: take them now.
			if sum, herr := hashFile(s.pdfPath(p)); herr == nil {
				p.SHA256, baselined = sum, true
			}
		}
	}
	entries, err := os.ReadDir(s.pdfDir())
	if err != nil {
		return baselined
	}
	changed := baselined
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.EqualFold(filepath.Ext(name), ".pdf") || known[strings.ToLower(name)] {
			continue
		}
		title := titleFromFile(name)
		sum, _ := hashFile(filepath.Join(s.pdfDir(), name))
		np := &Product{
			ID: newID(), Name: title, Category: catchAll, File: name, NeedsReview: true,
			AddedAt: now(), UpdatedAt: now(), PDFUpdatedAt: now(), SHA256: sum,
		}
		// A PDF dropped into the folder arrives with its summary already drafted for a manager to check.
		if d := draftFile(filepath.Join(s.pdfDir(), name)); d != nil {
			if d.Name != "" && regexp.MustCompile(`^[\d\s._-]*$|^sds\b`).MatchString(strings.ToLower(title)) {
				np.Name = d.Name
			}
			applyDraft(np, d)
		}
		s.lib.Products = append(s.lib.Products, np)
		changed = true
	}
	return changed
}

func (s *store) load() error {
	for _, d := range []string{s.dir, s.pdfDir(), s.goneDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	b, err := os.ReadFile(s.file())
	if errors.Is(err, fs.ErrNotExist) {
		s.lib = Library{Categories: defaultCategories, Locations: defaultLocations, Products: []*Product{}, Schema: 3}
		s.applyStarter()
		s.scan()
		if s.isReadOnly() {
			return nil
		}
		return s.save()
	}
	if err != nil {
		return err
	}
	var lib Library
	if err := json.Unmarshal(b, &lib); err != nil {
		return fmt.Errorf("library.json could not be read (%v). A copy from the previous save is in library.json.bak", err)
	}
	s.lib = lib
	if len(s.lib.Categories) == 0 {
		s.lib.Categories = defaultCategories
	}
	if s.lib.Products == nil {
		s.lib.Products = []*Product{}
	}
	upgraded := s.upgrade()
	if s.migrateLocations() {
		upgraded = true
	}
	starter := s.applyStarter()
	if (s.scan() || starter || upgraded) && !s.isReadOnly() {
		return s.save()
	}
	return nil
}

// save writes the library to a temporary file first so a crash or power cut
// can never leave a half-written library behind.
func (s *store) save() error {
	if s.isReadOnly() {
		return errReadOnly
	}
	b, err := json.MarshalIndent(s.lib, "", "  ")
	if err != nil {
		return err
	}
	s.safetyCopy(b)
	tmp := s.file() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if old, err := os.ReadFile(s.file()); err == nil {
		_ = os.WriteFile(s.file()+".bak", old, 0o644)
	}
	return os.Rename(tmp, s.file())
}

func (s *store) find(id string) *Product {
	for _, p := range s.lib.Products {
		if p.ID == id {
			return p
		}
	}
	return nil
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9]+`)

// archiveName picks an unused, readable file name in the "removed" folder.
func (s *store) archiveName(p *Product) string {
	name := strings.Trim(unsafeName.ReplaceAllString(p.Name, "-"), "-")
	if len(name) > 60 {
		name = name[:60]
	}
	return fmt.Sprintf("%s_%s_%s.pdf", name, time.Now().Format("2006-01-02_150405"), newID()[:6])
}

// readUpload parses the form. It returns the product fields and, when a PDF
// was attached, the path of a temporary file holding it.
type upload struct {
	Product
	Verify     bool   `json:"verify"`     // the manager ticked "checked against the SDS"
	VerifiedBy string `json:"verifiedBy"` // their name or initials
}

func (p *Product) summaryText() string {
	return strings.Join([]string{p.Hazards, p.FirstAidEyes, p.FirstAidSkin, p.FirstAidInhalation, p.FirstAidSwallowed, p.PPE,
		p.SignalWord, strings.Join(p.Pictograms, ",")}, "\x00")
}

func (p *Product) hasSummary() bool {
	return strings.TrimSpace(strings.ReplaceAll(p.summaryText(), "\x00", "")) != ""
}

// settleSummary works out the verification state of the emergency summary
// after a save. A summary only counts as verified when a manager says they
// checked it; editing the text or replacing the PDF takes that away.
func settleSummary(p *Product, in *upload, textChanged, pdfReplaced bool) {
	switch {
	case !p.hasSummary():
		p.SummaryVerifiedAt, p.SummaryVerifiedBy, p.SummarySourceRevision, p.SummaryNeedsCheck = "", "", "", false
	case in.Verify:
		p.SummaryVerifiedAt = time.Now().Format("2006-01-02")
		p.SummaryVerifiedBy = strings.TrimSpace(in.VerifiedBy)
		p.SummarySourceRevision = p.RevisionDate
		p.SummaryNeedsCheck = false
	case textChanged:
		p.SummaryVerifiedAt, p.SummaryVerifiedBy, p.SummarySourceRevision, p.SummaryNeedsCheck = "", "", "", false
	case pdfReplaced && p.SummaryVerifiedAt != "":
		p.SummaryNeedsCheck = true
	}
}

func (s *store) readUpload(w http.ResponseWriter, r *http.Request) (*upload, string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPDF)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, "", errors.New("the upload could not be read; the file may be too large")
	}
	var in upload
	if err := json.Unmarshal([]byte(r.FormValue("meta")), &in); err != nil {
		return nil, "", errors.New("the product details could not be read")
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, "", errors.New("a product name is required")
	}
	f, _, err := r.FormFile("pdf")
	if errors.Is(err, http.ErrMissingFile) {
		return &in, "", nil
	}
	if err != nil {
		return nil, "", errors.New("the PDF could not be read")
	}
	defer f.Close()
	head := make([]byte, 1024)
	n, _ := io.ReadFull(f, head)
	if !strings.Contains(string(head[:n]), "%PDF") {
		return nil, "", errors.New("that file is not a PDF")
	}
	tmp, err := os.CreateTemp(s.pdfDir(), "upload-*.tmp")
	if err != nil {
		return nil, "", err
	}
	defer tmp.Close()
	if _, err := tmp.Write(head[:n]); err == nil {
		_, err = io.Copy(tmp, f)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return nil, "", errors.New("the PDF could not be saved")
	}
	return &in, tmp.Name(), nil
}

func snapshot(p *Product) BinderItem {
	return BinderItem{p.Name, p.Manufacturer, p.Category, p.Location, p.RevisionDate, p.PDFUpdatedAt, p.SHA256}
}
