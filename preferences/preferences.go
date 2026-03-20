package preferences

import (
	"bufio"
	"fmt"
	"github.com/adrg/xdg"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type groupkey struct {
	group string
	key   string
}

type PreferencesFile struct {
	directory  string
	filename   string
	configFile string
	value      map[groupkey]any
}

func PreferencesFileNew(dir string, file string) *PreferencesFile {
	return &PreferencesFile{
		directory:  dir,
		filename:   file,
		configFile: "",
		value:      map[groupkey]any{groupkey{group: "", key: ""}: ""},
	}
}

func (pf *PreferencesFile) Set(g string, k string, value any) error {
	if _, defined := pf.value[groupkey{group: g, key: k}]; defined {
		delete(pf.value, groupkey{group: g, key: k})
	}

	if f0, ok := value.(float64); ok {
		pf.value[groupkey{group: g, key: k}] = f0
	} else if i0, ok := value.(int); ok {
		pf.value[groupkey{group: g, key: k}] = i0
	} else if b0, ok := value.(bool); ok {
		pf.value[groupkey{group: g, key: k}] = b0
	} else if v0, ok := value.(string); ok {
		if v0 == "true" {
			pf.value[groupkey{group: g, key: k}] = true
		} else if v0 == "false" {
			pf.value[groupkey{group: g, key: k}] = false
		} else if i, err := strconv.ParseInt(v0, 10, 64); err == nil {
			pf.value[groupkey{group: g, key: k}] = int(i)
		} else if f, err := strconv.ParseFloat(v0, 64); err == nil {
			pf.value[groupkey{group: g, key: k}] = f
		} else {
			pf.value[groupkey{group: g, key: k}] = v0
		}
	}
	return nil
}

func (pf *PreferencesFile) Get(g string, k string) (any, error) {
	if v, defined := pf.value[groupkey{group: g, key: k}]; defined {
		return v, nil
	}
	return nil, fmt.Errorf("undefined group %s or key %s", g, k)
}

func (pf *PreferencesFile) GetString(g string, k string) (string, error) {
	if v, defined := pf.value[groupkey{group: g, key: k}]; defined {
		if s, ok := v.(string); ok {
			return s, nil
		} else {
			return "", fmt.Errorf("type mismatch. %s's type is %T", v)
		}
	}
	return "", fmt.Errorf("undefined key %s", k)
}

func (pf *PreferencesFile) GetFloat(g string, k string) (float64, error) {
	if v, defined := pf.value[groupkey{group: g, key: k}]; defined {
		if f, ok := v.(float64); ok {
			return f, nil
		} else {
			return 0, fmt.Errorf("type mismatch. %s's type is %T", v)
		}
	}
	return 0, fmt.Errorf("undefined key %s", k)
}

func (pf *PreferencesFile) GetInt(g string, k string) (int, error) {
	if v, defined := pf.value[groupkey{group: g, key: k}]; defined {
		if i, ok := v.(int); ok {
			return i, nil
		} else {
			return 0, fmt.Errorf("type mismatch. %s's type is %T", v)
		}
	}
	return 0, fmt.Errorf("undefined key %s", k)
}

func (pf *PreferencesFile) GetBool(g string, k string) (bool, error) {
	if v, defined := pf.value[groupkey{group: g, key: k}]; defined {
		if b, ok := v.(bool); ok {
			return b, nil
		} else {
			return false, fmt.Errorf("type mismatch. %s's type is %T", v)
		}
	}
	return false, fmt.Errorf("undefined key %s", k)
}

func (pf *PreferencesFile) Load() error {
	fullpath, err := xdg.SearchConfigFile(pf.directory + "/" + pf.filename)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(fullpath, os.O_RDONLY, 0444)
	if err != nil {
		return err
	}
	pf.configFile = fullpath
	defer file.Close()

	scanner := bufio.NewScanner(file)
	group := ""
	// 一行ずつスキャン（改行コードは自動で除外される）
	for scanner.Scan() {
		line := scanner.Text() // string型で取得
		if line == "" {
			continue
		}
		switch line[0] {
		case '[':
			lp := strings.Index(line, "]")
			if lp > 1 {
				group = line[1:lp]
			}
			continue
		case '#':
			// コメント行 #
			continue
		default:
			v := strings.Split(line, "=")
			if len(v) < 2 {
				// コメント扱い
				continue
			}
			pf.Set(group, v[0], v[1])
		}
	}
	// スキャン中にエラーが発生したか確認
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func (pf *PreferencesFile) Save() error {
	if pf.configFile == "" {
		var err error
		for _, v := range []string{xdg.ConfigHome, xdg.Home} {
			dir := filepath.Join(v, pf.directory)
			err = os.Mkdir(dir, 0700)
			if err == nil || os.IsExist(err) {
				pf.configFile = filepath.Join(v, pf.directory, pf.filename)
				break
			}
		}
		if pf.configFile == "" {
			return err
		}
	}

	file, err := os.Create(pf.configFile)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	defer writer.Flush()

	var grouptable []string
	for k, _ := range pf.value {
		if k.group == "" {
			continue
		}
		if slices.Index(grouptable, k.group) == -1 {
			grouptable = append(grouptable, k.group)
		}
	}
	slices.Sort(grouptable)

	for _, gp := range grouptable {
		writer.WriteString(fmt.Sprintf("[%s]\n", gp))
		for k, v := range pf.value {
			if k.group == gp {
				writer.WriteString(fmt.Sprintf("%s=%v\n", k.key, v))
			}
		}
		writer.WriteString("\n")
	}

	return nil
}

func (pf *PreferencesFile) Dump() {
	var grouptable []string
	for k, _ := range pf.value {
		if k.group == "" {
			continue
		}
		if slices.Index(grouptable, k.group) == -1 {
			grouptable = append(grouptable, k.group)
		}
	}
	slices.Sort(grouptable)

	for _, gp := range grouptable {
		fmt.Printf("[%s]\n", gp)
		for k, v := range pf.value {
			if k.group == gp {
				fmt.Printf("%s=%v\n", k.key, v)
			}
		}
		fmt.Printf("\n")
	}
}
