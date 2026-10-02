package repository

import (
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

// SeedAds возвращает тестовые объявления, чтобы лента не была пустой после запуска.
func SeedAds() []domain.Ad {
	now := time.Now()

	ads := []domain.Ad{
		{CategoryID: 1, Title: "Велосипед горный Stels Navigator", Description: "Состояние отличное, 21 скорость", Price: 1500000, City: "Москва", HasDelivery: false},
		{CategoryID: 2, Title: "iPhone 13 128 ГБ", Description: "Без царапин, полный комплект", Price: 4500000, City: "Москва", HasDelivery: true},
		{CategoryID: 3, Title: "Диван угловой", Description: "Серый, раскладывается", Price: 2000000, City: "Химки", HasDelivery: false},
		{CategoryID: 2, Title: "Наушники Sony WH-1000XM4", Description: "Шумоподавление, кейс в комплекте", Price: 1800000, City: "Балашиха", HasDelivery: true},
		{CategoryID: 4, Title: "Куртка зимняя, размер M", Description: "Носил один сезон", Price: 350000, City: "Москва", HasDelivery: true},
		{CategoryID: 1, Title: "Самокат Xiaomi Mi Electric Scooter", Description: "Пробег 300 км", Price: 2200000, City: "Мытищи", HasDelivery: false},
		{CategoryID: 3, Title: "Стол письменный", Description: "Светлое дерево, 120×60", Price: 400000, City: "Москва", HasDelivery: false},
		{CategoryID: 4, Title: "Кроссовки Nike Air Max", Description: "Размер 42, новые", Price: 750000, City: "Химки", HasDelivery: true},
	}

	for i := range ads {
		ads[i].ID = int32(i + 1)
		ads[i].Status = domain.AdStatusActive
		ads[i].CreatedAt = now.Add(-time.Duration(i) * 3 * time.Hour)
	}

	return ads
}
