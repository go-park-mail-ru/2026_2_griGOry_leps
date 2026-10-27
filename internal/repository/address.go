package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/jackc/pgx/v5"
)

type AddressRepository struct {
	db DB
}

func NewAddressRepository(db DB) *AddressRepository {
	return &AddressRepository{db: db}
}

func scanAddress(row pgx.Row) (domain.Address, error) {
	var a domain.Address
	err := row.Scan(&a.ID, &a.UserID, &a.Street, &a.House, &a.Comment, &a.IsDefault, &a.CreatedAt)
	return a, err
}

func (r *AddressRepository) List(ctx context.Context, userID int32) ([]domain.Address, error) {
	rows, err := conn(ctx, r.db).Query(ctx, listAddressesQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("list addresses: %w", err)
	}
	defer rows.Close()

	addresses := make([]domain.Address, 0)
	for rows.Next() {
		a, err := scanAddress(rows)
		if err != nil {
			return nil, fmt.Errorf("scan address: %w", err)
		}
		addresses = append(addresses, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read addresses: %w", err)
	}
	return addresses, nil
}

func (r *AddressRepository) Count(ctx context.Context, userID int32) (int, error) {
	var count int
	if err := conn(ctx, r.db).QueryRow(ctx, countAddressesQuery, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count addresses: %w", err)
	}
	return count, nil
}

func (r *AddressRepository) GetForUpdate(ctx context.Context, userID, id int32) (domain.Address, error) {
	a, err := scanAddress(conn(ctx, r.db).QueryRow(ctx, getAddressForUpdateQuery, id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Address{}, domain.ErrAddressNotFound
	}
	if err != nil {
		return domain.Address{}, fmt.Errorf("get address: %w", err)
	}
	return a, nil
}

func (r *AddressRepository) Create(ctx context.Context, a domain.Address) (domain.Address, error) {
	created, err := scanAddress(conn(ctx, r.db).QueryRow(ctx, createAddressQuery,
		a.UserID, a.Street, a.House, a.Comment, a.IsDefault))
	if err != nil {
		return domain.Address{}, fmt.Errorf("create address: %w", err)
	}
	return created, nil
}

func (r *AddressRepository) Update(ctx context.Context, a domain.Address) (domain.Address, error) {
	updated, err := scanAddress(conn(ctx, r.db).QueryRow(ctx, updateAddressQuery,
		a.ID, a.UserID, a.Street, a.House, a.Comment, a.IsDefault))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Address{}, domain.ErrAddressNotFound
	}
	if err != nil {
		return domain.Address{}, fmt.Errorf("update address: %w", err)
	}
	return updated, nil
}

func (r *AddressRepository) Delete(ctx context.Context, userID, id int32) error {
	tag, err := conn(ctx, r.db).Exec(ctx, deleteAddressQuery, id, userID)
	if err != nil {
		return fmt.Errorf("delete address: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrAddressNotFound
	}
	return nil
}

func (r *AddressRepository) ClearDefault(ctx context.Context, userID int32) error {
	if _, err := conn(ctx, r.db).Exec(ctx, clearDefaultAddressQuery, userID); err != nil {
		return fmt.Errorf("clear default address: %w", err)
	}
	return nil
}

func (r *AddressRepository) PromoteOldest(ctx context.Context, userID, exceptID int32) error {
	if _, err := conn(ctx, r.db).Exec(ctx, promoteOldestAddressQuery, userID, exceptID); err != nil {
		return fmt.Errorf("promote oldest address: %w", err)
	}
	return nil
}
