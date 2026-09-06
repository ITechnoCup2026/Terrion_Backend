package planning

import (
	"fmt"
	"time"

	"terrion-backend/internal/agronomy"
)

type Season struct {
	Label        string
	Start        time.Time
	End          time.Time
	PlantingFrom time.Time
	PlantingTo   time.Time
}

func utcDay(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func SeasonMT1(startYear int) Season {
	return Season{
		Label:        fmt.Sprintf("MT I %d/%d", startYear, startYear+1),
		Start:        utcDay(startYear, time.October, 1),
		End:          utcDay(startYear+1, time.March, 31),
		PlantingFrom: utcDay(startYear, time.October, 1),
		PlantingTo:   utcDay(startYear, time.December, 31),
	}
}

func SeasonMT2(year int) Season {
	return Season{
		Label:        fmt.Sprintf("MT II %d", year),
		Start:        utcDay(year, time.April, 1),
		End:          utcDay(year, time.September, 30),
		PlantingFrom: utcDay(year, time.April, 1),
		PlantingTo:   utcDay(year, time.June, 30),
	}
}

func OpenSeasons(now time.Time) []Season {
	year := agronomy.StartOfDay(now).Year()

	candidates := []Season{
		SeasonMT1(year - 1), SeasonMT2(year),
		SeasonMT1(year), SeasonMT2(year + 1),
		SeasonMT1(year + 1),
	}

	open := []Season{}
	for _, season := range candidates {
		if season.PlantingTo.After(agronomy.StartOfDay(now)) {
			open = append(open, season)
		}
	}
	return open
}

func SeasonByLabel(label string, now time.Time) (Season, bool) {
	for _, season := range OpenSeasons(now) {
		if season.Label == label {
			return season, true
		}
	}
	return Season{}, false
}

// PlantingDateStepDays adalah jarak antar tanggal tanam yang ditawarkan
// perencana.
//
// Setiap tanggal dikalikan jumlah lahan dan jumlah varietas, jadi angka ini
// menentukan besar ruang pencarian secara langsung. Ia dinaikkan dari 7 ke 14
// ketika varietas acuan bertambah dari 13 menjadi 42: pada langkah mingguan
// satu koperasi berisi 14 lahan menghasilkan ~7.600 kombinasi, melewati batas
// 2.000 yang diterima layanan AI dan memberi solver lokal pekerjaan yang jauh
// lebih berat tanpa rencana yang lebih baik.
//
// Dua minggu juga lebih jujur terhadap cara orang menanam. Selisih tujuh hari
// pada tanggal tanam yang direncanakan tiga bulan di muka adalah presisi yang
// tidak bisa dipegang siapa pun begitu hujan datang terlambat.
const PlantingDateStepDays = 14

// CandidatePlantingDates menyusun tanggal tanam yang boleh ditawarkan untuk
// satu musim: mulai dari hari Senin pertama yang masih di depan, lalu
// melangkah PlantingDateStepDays sampai jendela tanam musim itu tutup.
func CandidatePlantingDates(season Season, now time.Time) []time.Time {
	earliest := agronomy.AddDays(agronomy.StartOfDay(now), 1)
	if season.PlantingFrom.After(earliest) {
		earliest = season.PlantingFrom
	}

	// Penyelarasan ke hari Senin, sekali di awal. Ini bukan langkahnya:
	// tanggal pertama harus jatuh pada awal minggu ISO, dan sesudah itu barulah
	// PlantingDateStepDays berlaku.
	cursor := agronomy.ISOWeekStart(earliest)
	if cursor.Before(earliest) {
		cursor = agronomy.AddDays(cursor, 7)
	}

	dates := []time.Time{}
	for !cursor.After(season.PlantingTo) {
		dates = append(dates, cursor)
		cursor = agronomy.AddDays(cursor, PlantingDateStepDays)
	}
	return dates
}

// PreviousSeason adalah musim sejenis setahun sebelumnya — MT I dibandingkan
// dengan MT I, bukan dengan MT II. Membandingkan musim yang berbeda jenis
// berarti membandingkan komoditas dan cuaca yang berbeda, dan angkanya
// menyesatkan justru pada hal yang paling ingin diketahui pengurus.
func PreviousSeason(season Season) Season {
	year := season.Start.Year()
	if season.Start.Month() == time.October {
		return SeasonMT1(year - 1)
	}
	return SeasonMT2(year - 1)
}
