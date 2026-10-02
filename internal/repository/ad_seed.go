package repository

import (
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

// SeedAds возвращает тестовые объявления, чтобы лента не была пустой после запуска.
func SeedAds() []domain.Ad {
	now := time.Now()

	ads := []domain.Ad{
		{CategoryID: 14, Title: "Велосипед горный, состояние отличное", Description: "Stels Navigator, 21 скорость", Price: 1500000, City: "Москва", HasDelivery: false, ImageURL: "/img/ads/bike-mountain.jpg"},
		{CategoryID: 14, Title: "Велосипед шоссейный, Б/У", Description: "Алюминиевая рама, размер M", Price: 4500000, City: "Москва", HasDelivery: false, ImageURL: "/img/ads/bike-road.jpg"},
		{CategoryID: 12, Title: "Рюкзак школьный ортопедический", Description: "Жёсткая спинка, светоотражатели", Price: 320000, City: "Химки", HasDelivery: true, ImageURL: "/img/ads/backpack.jpg"},
		{CategoryID: 4, Title: "Диван раскладной, доставка", Description: "Велюр, механизм еврокнижка", Price: 4800000, City: "Москва", HasDelivery: true, ImageURL: "/img/ads/sofa.jpg"},
		{CategoryID: 15, Title: "Комплект книг для 5 класса", Description: "Все учебники по программе", Price: 150000, City: "Балашиха", HasDelivery: true, ImageURL: "/img/ads/school-books.jpg"},
		{CategoryID: 7, Title: "Ноутбук игровой, гарантия", Description: "RTX 4060, 16 ГБ, гарантия до весны", Price: 6200000, City: "Москва", HasDelivery: true, ImageURL: "/img/ads/laptop.jpg"},
		{CategoryID: 14, Title: "Электросамокат Xiaomi, пробег 300 км", Description: "Родная зарядка, без ремонтов", Price: 1850000, City: "Мытищи", HasDelivery: true, ImageURL: "/img/ads/scooter.jpg"},
		{CategoryID: 14, Title: "Палатка 4-местная, двухслойная", Description: "Использовалась два похода", Price: 690000, City: "Москва", HasDelivery: true, ImageURL: "/img/ads/tent.jpg"},
		{CategoryID: 4, Title: "Кофемашина рожковая DeLonghi", Description: "Капучинатор, полный комплект", Price: 1400000, City: "Химки", HasDelivery: true, ImageURL: "/img/ads/coffee-machine.jpg"},
		{CategoryID: 16, Title: "Гитара акустическая Yamaha F310", Description: "Новые струны, чехол в подарок", Price: 950000, City: "Москва", HasDelivery: false, ImageURL: "/img/ads/guitar.jpg"},
		{CategoryID: 12, Title: "Коляска прогулочная, лёгкая", Description: "Складывается одной рукой", Price: 780000, City: "Балашиха", HasDelivery: true, ImageURL: "/img/ads/stroller.jpg"},
		{CategoryID: 7, Title: "PlayStation 5 с двумя джойстиками", Description: "Дисковая версия, на гарантии", Price: 4200000, City: "Москва", HasDelivery: false, ImageURL: "/img/ads/ps5.jpg"},
	}

	for i := range ads {
		ads[i].ID = int32(i + 1)
		ads[i].Status = domain.AdStatusActive
		ads[i].CreatedAt = now.Add(-time.Duration(i) * 3 * time.Hour)
	}

	return ads
}
