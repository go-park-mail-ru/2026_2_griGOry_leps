package repository

import (
	"context"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const searchCitiesQuery = `
 SELECT city_id, name
 FROM city
 WHERE $1::text='' OR starts_with(lower(name), lower($1))
 ORDER BY city_id
 LIMIT $2
`

type CityRepository struct {
	db DB
}

func NewCityRepository(db DB) *CityRepository {
	return &CityRepository{db: db}
}

func (r *CityRepository) Search(ctx context.Context, prefix string, limit int) ([]domain.City, error) {
	rows, err := r.db.Query(ctx, searchCitiesQuery, prefix, limit)
	if err != nil {
		return nil, fmt.Errorf("search cities: %w", err)
	}
	defer rows.Close()

	cities := make([]domain.City, 0)
	for rows.Next() {
		var c domain.City
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, fmt.Errorf("scan city: %w", err)
		}
		cities = append(cities, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read cities: %w", err)
	}
	return cities, nil
}
