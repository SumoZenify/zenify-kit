package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ZenifyAIContactCenter/zenify-kit/internal/dbread"
	"github.com/charmbracelet/huh"
	"golang.org/x/term"
)

var dbReadSetupProfiles = []string{"staging", "local", "develop", "custom", "production"}

type dbReadSetupPrompt func(initialProfile string, chooseProfile bool) (dbread.SetupInput, error)

func promptDBReadSetup(initialProfile string, chooseProfile bool) (dbread.SetupInput, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return dbread.SetupInput{}, errors.New("db-read setup needs an interactive terminal")
	}
	profile := initialProfile
	if chooseProfile {
		opts := make([]huh.Option[string], 0, len(dbReadSetupProfiles))
		for _, name := range dbReadSetupProfiles {
			opts = append(opts, huh.NewOption(name, name))
		}
		if err := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().Title("Profile cần cấu hình").Options(opts...).Value(&profile), //znf:allow-lang
		)).Run(); err != nil {
			return dbread.SetupInput{}, err
		}
	}
	if !isDBReadSetupProfile(profile) {
		return dbread.SetupInput{}, fmt.Errorf("unsupported setup profile %q: choose staging, local, develop, custom, or production", profile)
	}

	engine := "both"
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Database cần cấu hình"). //znf:allow-lang
			Options(
				huh.NewOption("MongoDB và MySQL", "both"), //znf:allow-lang
				huh.NewOption("MongoDB", "mongo"),
				huh.NewOption("MySQL", "mysql"),
			).
			Value(&engine),
	)).Run(); err != nil {
		return dbread.SetupInput{}, err
	}

	mongoURIEnv := defaultProfileEnv("MONGO_URL", profile)
	mongoDatabase := ""
	mongoURI := ""
	mysqlHostEnv := defaultProfileEnv("MYSQL_HOST", profile)
	mysqlPortEnv := defaultProfileEnv("MYSQL_PORT", profile)
	mysqlUserEnv := defaultProfileEnv("MYSQL_USER", profile)
	mysqlPasswordEnv := defaultProfileEnv("MYSQL_PASSWORD", profile)
	mysqlDatabase := ""
	mysqlHost := ""
	mysqlPort := ""
	mysqlUser := ""
	mysqlPassword := ""

	fields := make([]huh.Field, 0, 12)
	if engine == "both" || engine == "mongo" {
		fields = append(fields,
			huh.NewInput().Title("Mongo URI env key").Value(&mongoURIEnv).Validate(requiredEnvKey),
			huh.NewInput().Title("Mongo database").Value(&mongoDatabase).Validate(requiredValue),
			huh.NewInput().Title("Mongo URI").Description("Masked; để trống để giữ giá trị hiện có").EchoMode(huh.EchoModePassword).Value(&mongoURI), //znf:allow-lang
		)
	}
	if engine == "both" || engine == "mysql" {
		fields = append(fields,
			huh.NewInput().Title("MySQL host env key").Value(&mysqlHostEnv).Validate(requiredEnvKey),
			huh.NewInput().Title("MySQL port env key").Value(&mysqlPortEnv).Validate(requiredEnvKey),
			huh.NewInput().Title("MySQL user env key").Value(&mysqlUserEnv).Validate(requiredEnvKey),
			huh.NewInput().Title("MySQL password env key").Value(&mysqlPasswordEnv).Validate(requiredEnvKey),
			huh.NewInput().Title("MySQL database").Value(&mysqlDatabase).Validate(requiredValue),
			huh.NewInput().Title("MySQL host").Description("Để trống để giữ giá trị hiện có").Value(&mysqlHost),                                                //znf:allow-lang
			huh.NewInput().Title("MySQL port").Description("Để trống để giữ giá trị hiện có hoặc dùng 3306").Value(&mysqlPort),                                 //znf:allow-lang
			huh.NewInput().Title("MySQL user").Description("Để trống để giữ giá trị hiện có").Value(&mysqlUser),                                                //znf:allow-lang
			huh.NewInput().Title("MySQL password").Description("Masked; để trống để giữ giá trị hiện có").EchoMode(huh.EchoModePassword).Value(&mysqlPassword), //znf:allow-lang
		)
	}
	if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
		return dbread.SetupInput{}, err
	}

	if profile == "production" {
		confirmed := false
		if err := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().Title("Lưu profile production với tài khoản chỉ đọc?").Value(&confirmed), //znf:allow-lang
		)).Run(); err != nil {
			return dbread.SetupInput{}, err
		}
		if !confirmed {
			return dbread.SetupInput{}, huh.ErrUserAborted
		}
	}

	input := dbread.SetupInput{
		Name:        profile,
		Credentials: map[string]string{},
	}
	if engine == "both" || engine == "mongo" {
		input.Profile.Mongo = &dbread.MongoProfile{URIEnv: strings.TrimSpace(mongoURIEnv), Database: strings.TrimSpace(mongoDatabase)}
		input.Credentials[input.Profile.Mongo.URIEnv] = mongoURI
	}
	if engine == "both" || engine == "mysql" {
		input.Profile.MySQL = &dbread.MySQLProfile{
			HostEnv:     strings.TrimSpace(mysqlHostEnv),
			PortEnv:     strings.TrimSpace(mysqlPortEnv),
			UserEnv:     strings.TrimSpace(mysqlUserEnv),
			PasswordEnv: strings.TrimSpace(mysqlPasswordEnv),
			Database:    strings.TrimSpace(mysqlDatabase),
		}
		input.Credentials[input.Profile.MySQL.HostEnv] = strings.TrimSpace(mysqlHost)
		input.Credentials[input.Profile.MySQL.PortEnv] = strings.TrimSpace(mysqlPort)
		input.Credentials[input.Profile.MySQL.UserEnv] = strings.TrimSpace(mysqlUser)
		input.Credentials[input.Profile.MySQL.PasswordEnv] = mysqlPassword
	}
	return input, nil
}

func defaultProfileEnv(base, profile string) string {
	if profile == "staging" {
		return base
	}
	return base + "_" + strings.ToUpper(strings.ReplaceAll(profile, "-", "_"))
}

func isDBReadSetupProfile(profile string) bool {
	for _, allowed := range dbReadSetupProfiles {
		if profile == allowed {
			return true
		}
	}
	return false
}

func requiredValue(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("value is required")
	}
	return nil
}

func requiredEnvKey(value string) error {
	value = strings.TrimSpace(value)
	if err := requiredValue(value); err != nil {
		return err
	}
	for i, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return errors.New("use letters, numbers, and underscore; do not start with a number")
	}
	return nil
}
