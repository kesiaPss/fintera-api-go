package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"gorm.io/gorm"
)

// ErrHistoricalClientStaffIdentity is returned when a historical client's
// identity belongs to an admin or seller account.
var ErrHistoricalClientStaffIdentity = errors.New("la identidad del cliente histórico pertenece a un usuario del staff")

const historicalClientEmailDomain = "fintera.local"

// HistoricalClientBackfillResult summarizes a deployment backfill run.
type HistoricalClientBackfillResult struct {
	Linked   int
	Skipped  int
	Errors   int
	Messages []string
}

// HistoricalClientService turns historical sales into real customer accounts so
// they can appear in the admin Users list and in the reservation search.
type HistoricalClientService struct {
	users repository.UserRepository
	sales repository.HistoricalSaleRepository
}

// NewHistoricalClientService creates the historical-client linker.
func NewHistoricalClientService(users repository.UserRepository, sales repository.HistoricalSaleRepository) *HistoricalClientService {
	return &HistoricalClientService{users: users, sales: sales}
}

// EnsureLinked resolves or creates the customer account for a historical sale
// and stores its id on the record.
func (s *HistoricalClientService) EnsureLinked(ctx context.Context, sale *models.HistoricalSale, actorID uint) error {
	if s == nil || s.users == nil || sale == nil {
		return nil
	}
	user, err := s.resolve(ctx, sale, actorID)
	if err != nil {
		return err
	}
	sale.ClientUserID = &user.ID
	return nil
}

// SyncUser keeps the contact data of a historical account aligned with an edited
// sale. Identity changes are handled by EnsureLinked; this only refreshes the
// mutable contact fields on accounts created by the migration.
func (s *HistoricalClientService) SyncUser(ctx context.Context, sale *models.HistoricalSale) error {
	if s == nil || s.users == nil || sale == nil || sale.ClientUserID == nil {
		return nil
	}
	user, err := s.users.FindByID(ctx, *sale.ClientUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if !user.HistoricalClient {
		return nil
	}
	user.FullName = strings.TrimSpace(sale.ClientName)
	user.Phone = strings.TrimSpace(derefHistoricalString(sale.ClientPhone))
	email := strings.TrimSpace(derefHistoricalString(sale.ClientEmail))
	if email != "" && isHistoricalPlaceholderEmail(user.Email) {
		if existing, findErr := s.users.FindByEmail(ctx, email); findErr == nil && existing != nil && existing.ID != user.ID {
			// Keep the placeholder rather than stealing another account's email.
		} else {
			user.Email = strings.ToLower(email)
		}
	}
	return s.users.Update(ctx, user)
}

// Backfill links every unlinked historical sale in id order. It is safe to run
// on every deployment: linked rows are skipped.
func (s *HistoricalClientService) Backfill(ctx context.Context, batchSize int) (HistoricalClientBackfillResult, error) {
	result := HistoricalClientBackfillResult{}
	if s == nil || s.users == nil || s.sales == nil {
		return result, nil
	}
	if batchSize <= 0 {
		batchSize = 200
	}
	afterID := uint(0)
	for {
		sales, err := s.sales.ListWithoutClientUser(ctx, afterID, batchSize)
		if err != nil {
			return result, err
		}
		if len(sales) == 0 {
			return result, nil
		}
		for i := range sales {
			sale := &sales[i]
			afterID = sale.ID
			user, resolveErr := s.resolve(ctx, sale, sale.CreatedBy)
			if resolveErr != nil {
				result.Skipped++
				result.Errors++
				result.Messages = append(result.Messages, fmt.Sprintf("sale %d: %v", sale.ID, resolveErr))
				continue
			}
			if err := s.sales.SetClientUser(ctx, sale.ID, user.ID); err != nil {
				result.Errors++
				result.Messages = append(result.Messages, fmt.Sprintf("sale %d: %v", sale.ID, err))
				continue
			}
			result.Linked++
		}
		if len(sales) < batchSize {
			return result, nil
		}
	}
}

func (s *HistoricalClientService) resolve(ctx context.Context, sale *models.HistoricalSale, actorID uint) (*models.User, error) {
	identity := strings.TrimSpace(derefHistoricalString(sale.ClientIdentity))
	if identity == "" {
		return nil, ErrHistoricalSaleInvalid
	}

	if user, err := s.users.FindByIdentity(ctx, identity); err == nil && user != nil {
		if user.IsAdmin() || user.IsSeller() {
			return nil, ErrHistoricalClientStaffIdentity
		}
		return user, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	email := strings.TrimSpace(derefHistoricalString(sale.ClientEmail))
	if email != "" {
		if user, err := s.users.FindByEmail(ctx, email); err == nil && user != nil {
			if user.IsAdmin() || user.IsSeller() {
				// The email belongs to staff. Fall back to a placeholder account.
			} else if strings.EqualFold(strings.TrimSpace(user.Identity), identity) {
				return user, nil
			}
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	user, err := s.create(ctx, sale, actorID, email)
	if err != nil {
		if existing, findErr := s.users.FindByIdentity(ctx, identity); findErr == nil && existing != nil {
			return existing, nil
		}
		return nil, err
	}
	return user, nil
}

func (s *HistoricalClientService) create(ctx context.Context, sale *models.HistoricalSale, actorID uint, email string) (*models.User, error) {
	identity := strings.TrimSpace(derefHistoricalString(sale.ClientIdentity))
	targetEmail := strings.ToLower(email)
	if targetEmail == "" {
		targetEmail = historicalPlaceholderEmail(identity)
	}

	password, err := GenerateTempPassword()
	if err != nil {
		return nil, err
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Email:              targetEmail,
		EncryptedPassword:  hashed,
		Role:               models.RoleUser,
		Status:             models.StatusActive,
		FullName:           strings.TrimSpace(sale.ClientName),
		Phone:              strings.TrimSpace(derefHistoricalString(sale.ClientPhone)),
		Identity:           identity,
		Locale:             models.LocaleES,
		HistoricalClient:   true,
		MustChangePassword: true,
		EmailNotifications: false,
	}
	if actorID > 0 {
		user.CreatedBy = &actorID
	}
	note := "Cliente histórico migrado"
	user.Note = &note

	if err := s.users.CreateHistoricalClient(ctx, user); err != nil {
		// A concurrent create, a taken email, or two identities that normalize
		// to the same placeholder: retry once with a unique suffix.
		user.Email = historicalPlaceholderEmailWithSuffix(identity)
		if retryErr := s.users.CreateHistoricalClient(ctx, user); retryErr == nil {
			return user, nil
		}
		return nil, err
	}
	return user, nil
}

func historicalPlaceholderEmail(identity string) string {
	return fmt.Sprintf("historico+%s@%s", normalizedIdentity(identity), historicalClientEmailDomain)
}

func historicalPlaceholderEmailWithSuffix(identity string) string {
	return fmt.Sprintf("historico+%s-%d@%s", normalizedIdentity(identity), time.Now().UnixNano(), historicalClientEmailDomain)
}

func normalizedIdentity(identity string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(identity) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	local := b.String()
	if local == "" {
		local = "cliente"
	}
	return local
}

func isHistoricalPlaceholderEmail(email string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(email)), "@"+historicalClientEmailDomain)
}
