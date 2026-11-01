package repository

import "database/sql"

func collectRows[T any](rows *sql.Rows, scan func(scanner) (T, error)) ([]T, error) {
	defer rows.Close()
	result := make([]T, 0)
	for rows.Next() {
		value, err := scan(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func scanValue[T any](row scanner) (T, error) {
	var value T
	err := row.Scan(&value)
	return value, err
}
