package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/config"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/supabase"
)

type account struct {
	role         constants.UserRole
	email        string
	fullName     string
	organisation string
	password     string
}

type cooperative struct {
	id       string
	create   bool
	name     string
	village  string
	district string
	province string
	lat      float64
	lng      float64
}

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func main() {
	role := flag.String("role", "", "pengurus, kader or buyer")
	email := flag.String("email", "", "the account's email address")
	fullName := flag.String("name", "", "the person's full name")
	organisation := flag.String("organisation", "", "the buyer's organisation")
	password := flag.String("password", "", "leave empty to generate one")

	cooperativeID := flag.String("cooperative", "", "id of an existing cooperative")
	create := flag.Bool("create-cooperative", false, "register a new cooperative for a pengurus")
	coopName := flag.String("coop-name", "", "name of the new cooperative")
	village := flag.String("village", "", "village of the new cooperative")
	district := flag.String("district", "", "district of the new cooperative")
	province := flag.String("province", "", "province of the new cooperative")
	lat := flag.Float64("lat", 0, "latitude of the new cooperative")
	lng := flag.Float64("lng", 0, "longitude of the new cooperative")

	flag.Usage = usage
	flag.Parse()

	requested := account{
		role:         constants.UserRole(*role),
		email:        strings.ToLower(strings.TrimSpace(*email)),
		fullName:     strings.TrimSpace(*fullName),
		organisation: strings.TrimSpace(*organisation),
		password:     *password,
	}
	home := cooperative{
		id:       *cooperativeID,
		create:   *create,
		name:     strings.TrimSpace(*coopName),
		village:  strings.TrimSpace(*village),
		district: strings.TrimSpace(*district),
		province: strings.TrimSpace(*province),
		lat:      *lat,
		lng:      *lng,
	}

	cfg := config.NewConfig()
	log := config.NewLogger(cfg)

	if err := check(requested, home); err != nil {
		flag.Usage()
		log.Fatalf("%v", err)
	}
	if requested.password == "" {
		generated, err := generatePassword()
		if err != nil {
			log.Fatalf("generating a password: %v", err)
		}
		requested.password = generated
	}

	db := config.NewDatabase(cfg, log)
	goTrue := supabase.NewClient(
		cfg.Supabase.URL, cfg.Supabase.AnonKey, cfg.Supabase.ServiceRoleKey)

	if err := register(context.Background(), db, goTrue, log, requested, home); err != nil {
		log.Fatalf("%v", err)
	}
}

func check(requested account, home cooperative) error {
	switch requested.role {
	case constants.RolePengurus, constants.RoleKader, constants.RoleBuyer:
	default:
		return fmt.Errorf("role must be pengurus, kader or buyer, got %q", requested.role)
	}
	if !emailPattern.MatchString(requested.email) {
		return fmt.Errorf("%q does not look like an email address", requested.email)
	}
	if len(requested.fullName) < 2 {
		return errors.New("the account needs a full name")
	}

	if requested.role == constants.RoleBuyer {
		if home.id != "" || home.create {
			return errors.New("a buyer belongs to no cooperative")
		}
		return nil
	}

	if home.id == "" && !home.create {
		return errors.New("pass -cooperative <id>, or -create-cooperative with its details")
	}
	if home.id != "" && home.create {
		return errors.New("pass either -cooperative or -create-cooperative, not both")
	}
	if !home.create {
		return nil
	}

	if requested.role == constants.RoleKader {
		return errors.New(
			"a kader joins an existing cooperative; register its pengurus first")
	}
	if len(home.name) < 3 || home.village == "" || home.district == "" || home.province == "" {
		return errors.New("a new cooperative needs -coop-name, -village, -district and -province")
	}
	if home.lat < constants.IndonesiaMinLat || home.lat > constants.IndonesiaMaxLat ||
		home.lng < constants.IndonesiaMinLng || home.lng > constants.IndonesiaMaxLng {
		return errors.New("-lat and -lng must fall inside Indonesia")
	}
	return nil
}

func register(
	ctx context.Context, db *gorm.DB, goTrue *supabase.Client, log *logrus.Logger,
	requested account, home cooperative,
) error {
	if home.create {
		log.Warn("registering a cooperative asserts that it exists and that you have " +
			"verified it offline")
	}

	userID, err := goTrue.CreateUser(ctx, requested.email, requested.password)
	if err != nil {
		return fmt.Errorf("creating the auth user: %w", err)
	}

	if err := persist(ctx, db, userID, requested, home); err != nil {
		if deleteErr := goTrue.DeleteUser(ctx, userID); deleteErr != nil {
			log.Errorf("orphaned auth user %s: %v", userID, deleteErr)
		}
		return err
	}

	log.Infof("registered %s as %s", requested.email, requested.role)
	fmt.Printf("email:    %s\npassword: %s\n", requested.email, requested.password)
	return nil
}

func persist(
	ctx context.Context, db *gorm.DB, userID string, requested account, home cooperative,
) error {
	tx := db.WithContext(ctx).Begin()
	defer tx.Rollback()

	cooperativeID, err := resolveCooperative(tx, home)
	if err != nil {
		return err
	}

	profile := &entity.AppUser{
		ID:            userID,
		Role:          requested.role,
		CooperativeID: cooperativeID,
		FullName:      requested.fullName,
	}
	if requested.organisation != "" {
		profile.Organisation = &requested.organisation
	}

	if err := tx.Create(profile).Error; err != nil {
		return fmt.Errorf("creating the profile of %s: %w", requested.email, err)
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("committing the registration of %s: %w", requested.email, err)
	}
	return nil
}

func resolveCooperative(tx *gorm.DB, home cooperative) (*string, error) {
	if home.id != "" {
		existing := new(entity.Cooperative)
		if err := tx.Where("id = ?", home.id).Take(existing).Error; err != nil {
			return nil, fmt.Errorf("reading cooperative %s: %w", home.id, err)
		}
		return &existing.ID, nil
	}
	if !home.create {
		return nil, nil
	}

	created := &entity.Cooperative{
		ID:             uuid.NewString(),
		Name:           home.name,
		Village:        home.village,
		District:       home.district,
		Province:       home.province,
		Lat:            home.lat,
		Lng:            home.lng,
		StaggerApplied: json.RawMessage("[]"),
	}
	if err := tx.Create(created).Error; err != nil {
		return nil, fmt.Errorf("creating cooperative %q: %w", home.name, err)
	}
	return &created.ID, nil
}

func generatePassword() (string, error) {
	raw := make([]byte, constants.GeneratedPasswordBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return constants.GeneratedPasswordPrefix + hex.EncodeToString(raw), nil
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: go run cmd/register/main.go -role <role> -email <email> -name <name> [flags]

Terrion has no signup form for cooperatives on purpose: a cooperative is a legal
entity, and no form can tell a real one from a name typed into a box. This is
what turns an offline check into an account.

  -role pengurus     runs a cooperative: commits it to orders, answers buyers
  -role kader        registers land for a cooperative, cannot commit it
  -role buyer        browses the catalogue, belongs to no cooperative

A pengurus either joins an existing cooperative or brings a new one:

  -cooperative <id>
  -create-cooperative -coop-name .. -village .. -district .. -province .. -lat .. -lng ..

A kader always joins an existing cooperative. A password is generated unless
-password is given, and printed once.

`)
	flag.PrintDefaults()
}
