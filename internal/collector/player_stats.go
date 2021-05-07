package collector

import (
	"strings"

	"github.com/kinduff/csgo_exporter/internal/client"
	"github.com/kinduff/csgo_exporter/internal/model"

	"github.com/prometheus/client_golang/prometheus"

	log "github.com/sirupsen/logrus"
)

type playerCollector struct {
	config             *model.Config
	statsMetric        *prometheus.Desc
	achievementsMetric *prometheus.Desc
	newsMetric         *prometheus.Desc
}

// NewPlayerCollector provides an interface to collector player statistics.
func NewPlayerCollector(config *model.Config) *playerCollector {
	return &playerCollector{
		config: config,
		statsMetric: prometheus.NewDesc("stats_metric",
			"Shows metrics a player has from all its matches",
			[]string{"name", "player"},
			nil,
		),
		achievementsMetric: prometheus.NewDesc("achievements_metric",
			"Shows all the achievements from a player",
			[]string{"name", "player", "title", "description"},
			nil,
		),
		newsMetric: prometheus.NewDesc("news_metric",
			"Shows the latest news from CSGO",
			[]string{"title", "url", "feedlabel"},
			nil,
		),
	}
}

func (collector *playerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.statsMetric
	ch <- collector.achievementsMetric
	ch <- collector.newsMetric
}

func (collector *playerCollector) Collect(ch chan<- prometheus.Metric) {
	var allPlayerAchievementsDetails = map[string]map[string]string{}
	var allPlayerAchievements = map[string]int{}

	client := client.NewClient()

	if collector.config.SteamID == "" {
		ResolveVanityUrl := model.ResolveVanityUrl{}
		if err := client.DoRequest("id", collector.config, &ResolveVanityUrl); err != nil {
			log.Fatal(err)
		}
		collector.config.SteamID = ResolveVanityUrl.Response.Steamid
	}

	playerStats := model.PlayerStats{}
	if err := client.DoRequest("stats", collector.config, &playerStats); err != nil {
		log.Fatal(err)
	}

	archivements := model.Achievements{}
	if err := client.DoRequest("achievements", collector.config, &archivements); err != nil {
		log.Fatal(err)
	}

	news := model.News{}
	if err := client.DoRequest("news", collector.config, &news); err != nil {
		log.Fatal(err)
	}

	for _, s := range archivements.AchievementPercentages.Achievements {
		allPlayerAchievements[s.Name] = 0
	}

	playerAchievements := playerStats.PlayerStats.Achievements
	for _, s := range playerAchievements {
		allPlayerAchievements[s.Name] = 1
	}

	player := collector.config.SteamName
	if player == "" {
		player = collector.config.SteamID
	}

	achievementsDetails := model.AchievementsDetails{}
	if err := client.DoXMLRequest(collector.config, &achievementsDetails); err != nil {
		log.Fatal(err)
	}

	for _, s := range achievementsDetails.Achievements.Achievement {
		inner, ok := allPlayerAchievementsDetails[s.Apiname]
		if !ok {
			inner = make(map[string]string)
			allPlayerAchievementsDetails[s.Apiname] = inner
		}
		inner["title"] = s.Name
		inner["description"] = s.Description
	}

	for _, s := range playerStats.PlayerStats.Stats {
		if strings.Contains(s.Name, "GI") {
			continue
		}

		ch <- prometheus.MustNewConstMetric(collector.statsMetric, prometheus.GaugeValue, float64(s.Value), s.Name, player)
	}

	for name, count := range allPlayerAchievements {
		ch <- prometheus.MustNewConstMetric(collector.achievementsMetric, prometheus.GaugeValue, float64(count), name, player, allPlayerAchievementsDetails[strings.ToLower(name)]["title"], allPlayerAchievementsDetails[strings.ToLower(name)]["description"])
	}

	for _, s := range news.Appnews.Newsitems {
		ch <- prometheus.MustNewConstMetric(collector.newsMetric, prometheus.GaugeValue, float64(s.Date)*1000, s.Title, s.URL, s.Feedlabel)
	}
}
