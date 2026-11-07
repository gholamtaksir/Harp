package core

// colo is a Cloudflare data center, named after the nearest airport.
type colo struct {
	City    string
	Country string
}

// colos covers the data centers WARP users commonly land in. Unknown codes
// are shown as they are.
var colos = map[string]colo{
	// Europe
	"AMS": {"Amsterdam", "NL"}, "ARN": {"Stockholm", "SE"}, "ATH": {"Athens", "GR"},
	"BCN": {"Barcelona", "ES"}, "BEG": {"Belgrade", "RS"}, "BRU": {"Brussels", "BE"},
	"BUD": {"Budapest", "HU"}, "CDG": {"Paris", "FR"}, "CPH": {"Copenhagen", "DK"},
	"DUB": {"Dublin", "IE"}, "DUS": {"Düsseldorf", "DE"}, "EDI": {"Edinburgh", "GB"},
	"FCO": {"Rome", "IT"}, "FRA": {"Frankfurt", "DE"}, "HAM": {"Hamburg", "DE"},
	"HEL": {"Helsinki", "FI"}, "KBP": {"Kyiv", "UA"}, "KIV": {"Chișinău", "MD"},
	"LHR": {"London", "GB"}, "LIS": {"Lisbon", "PT"}, "LUX": {"Luxembourg", "LU"},
	"MAD": {"Madrid", "ES"}, "MAN": {"Manchester", "GB"}, "MRS": {"Marseille", "FR"},
	"MUC": {"Munich", "DE"}, "MXP": {"Milan", "IT"}, "OSL": {"Oslo", "NO"},
	"OTP": {"Bucharest", "RO"}, "PRG": {"Prague", "CZ"}, "RIX": {"Riga", "LV"},
	"SKG": {"Thessaloniki", "GR"}, "SOF": {"Sofia", "BG"}, "TLL": {"Tallinn", "EE"},
	"VIE": {"Vienna", "AT"}, "VNO": {"Vilnius", "LT"}, "WAW": {"Warsaw", "PL"},
	"ZAG": {"Zagreb", "HR"}, "ZRH": {"Zurich", "CH"}, "LED": {"Saint Petersburg", "RU"},
	"DME": {"Moscow", "RU"}, "SVX": {"Yekaterinburg", "RU"}, "KJA": {"Krasnoyarsk", "RU"},
	"IST": {"Istanbul", "TR"}, "ADB": {"Izmir", "TR"}, "ESB": {"Ankara", "TR"},
	"TBS": {"Tbilisi", "GE"}, "EVN": {"Yerevan", "AM"}, "GYD": {"Baku", "AZ"},
	"LCA": {"Nicosia", "CY"}, "MLA": {"Valletta", "MT"}, "PMO": {"Palermo", "IT"},
	"BTS": {"Bratislava", "SK"}, "LJU": {"Ljubljana", "SI"}, "TIA": {"Tirana", "AL"},
	"SKP": {"Skopje", "MK"}, "KEF": {"Reykjavík", "IS"}, "GOT": {"Gothenburg", "SE"},
	"STR": {"Stuttgart", "DE"}, "BER": {"Berlin", "DE"}, "TXL": {"Berlin", "DE"},
	"LYS": {"Lyon", "FR"}, "PMI": {"Palma", "ES"},

	// Middle East and Central Asia
	"DXB": {"Dubai", "AE"}, "AUH": {"Abu Dhabi", "AE"}, "FJR": {"Fujairah", "AE"},
	"DOH": {"Doha", "QA"}, "BAH": {"Manama", "BH"}, "KWI": {"Kuwait City", "KW"},
	"MCT": {"Muscat", "OM"}, "RUH": {"Riyadh", "SA"}, "JED": {"Jeddah", "SA"},
	"DMM": {"Dammam", "SA"}, "AMM": {"Amman", "JO"}, "BEY": {"Beirut", "LB"},
	"TLV": {"Tel Aviv", "IL"}, "HFA": {"Haifa", "IL"}, "ZDM": {"Ramallah", "PS"},
	"BGW": {"Baghdad", "IQ"}, "BSR": {"Basra", "IQ"}, "EBL": {"Erbil", "IQ"},
	"NJF": {"Najaf", "IQ"}, "ISU": {"Sulaymaniyah", "IQ"}, "XNH": {"Nasiriyah", "IQ"},
	"ALA": {"Almaty", "KZ"}, "NQZ": {"Astana", "KZ"}, "TAS": {"Tashkent", "UZ"},
	"FRU": {"Bishkek", "KG"}, "DYU": {"Dushanbe", "TJ"}, "ASB": {"Ashgabat", "TM"},

	// South and East Asia
	"BOM": {"Mumbai", "IN"}, "DEL": {"New Delhi", "IN"}, "MAA": {"Chennai", "IN"},
	"BLR": {"Bangalore", "IN"}, "HYD": {"Hyderabad", "IN"}, "CCU": {"Kolkata", "IN"},
	"KHI": {"Karachi", "PK"}, "LHE": {"Lahore", "PK"}, "ISB": {"Islamabad", "PK"},
	"DAC": {"Dhaka", "BD"}, "CMB": {"Colombo", "LK"}, "KTM": {"Kathmandu", "NP"},
	"KBL": {"Kabul", "AF"}, "SIN": {"Singapore", "SG"}, "HKG": {"Hong Kong", "HK"},
	"NRT": {"Tokyo", "JP"}, "KIX": {"Osaka", "JP"}, "FUK": {"Fukuoka", "JP"},
	"ICN": {"Seoul", "KR"}, "TPE": {"Taipei", "TW"}, "MNL": {"Manila", "PH"},
	"BKK": {"Bangkok", "TH"}, "KUL": {"Kuala Lumpur", "MY"}, "CGK": {"Jakarta", "ID"},
	"SGN": {"Ho Chi Minh City", "VN"}, "HAN": {"Hanoi", "VN"}, "PNH": {"Phnom Penh", "KH"},
	"ULN": {"Ulaanbaatar", "MN"}, "MFM": {"Macau", "MO"},

	// Africa
	"CAI": {"Cairo", "EG"}, "JNB": {"Johannesburg", "ZA"}, "CPT": {"Cape Town", "ZA"},
	"DUR": {"Durban", "ZA"}, "LOS": {"Lagos", "NG"}, "NBO": {"Nairobi", "KE"},
	"ALG": {"Algiers", "DZ"}, "TUN": {"Tunis", "TN"}, "CMN": {"Casablanca", "MA"},
	"ACC": {"Accra", "GH"}, "ADD": {"Addis Ababa", "ET"}, "DAR": {"Dar es Salaam", "TZ"},
	"KGL": {"Kigali", "RW"}, "MBA": {"Mombasa", "KE"}, "DKR": {"Dakar", "SN"},

	// Americas
	"ATL": {"Atlanta", "US"}, "BOS": {"Boston", "US"}, "ORD": {"Chicago", "US"},
	"DFW": {"Dallas", "US"}, "DEN": {"Denver", "US"}, "IAD": {"Ashburn", "US"},
	"LAX": {"Los Angeles", "US"}, "MIA": {"Miami", "US"}, "EWR": {"Newark", "US"},
	"SJC": {"San Jose", "US"}, "SEA": {"Seattle", "US"}, "PHX": {"Phoenix", "US"},
	"IAH": {"Houston", "US"}, "MSP": {"Minneapolis", "US"}, "LAS": {"Las Vegas", "US"},
	"YYZ": {"Toronto", "CA"}, "YUL": {"Montréal", "CA"}, "YVR": {"Vancouver", "CA"},
	"GRU": {"São Paulo", "BR"}, "GIG": {"Rio de Janeiro", "BR"}, "EZE": {"Buenos Aires", "AR"},
	"SCL": {"Santiago", "CL"}, "BOG": {"Bogotá", "CO"}, "LIM": {"Lima", "PE"},
	"MEX": {"Mexico City", "MX"}, "QRO": {"Querétaro", "MX"},

	// Oceania
	"SYD": {"Sydney", "AU"}, "MEL": {"Melbourne", "AU"}, "BNE": {"Brisbane", "AU"},
	"PER": {"Perth", "AU"}, "AKL": {"Auckland", "NZ"},
}

// ColoLocation returns the city and country of a data center code, or
// empty strings when it is unknown.
func ColoLocation(code string) (city, country string) {
	if info, ok := colos[code]; ok {
		return info.City, info.Country
	}
	return "", ""
}
