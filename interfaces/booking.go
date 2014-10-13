package interfaces

import (
	"fmt"
	"strconv"
	"time"

	"github.com/marcusolsson/goddd/application"
	"github.com/marcusolsson/goddd/domain/cargo"
	"github.com/marcusolsson/goddd/domain/location"
	"github.com/marcusolsson/goddd/domain/voyage"
)

type cargoDTO struct {
	TrackingId           string     `json:"trackingId"`
	StatusText           string     `json:"statusText"`
	Origin               string     `json:"origin"`
	Destination          string     `json:"destination"`
	ETA                  string     `json:"eta"`
	NextExpectedActivity string     `json:"nextExpectedActivity"`
	Misrouted            bool       `json:"misrouted"`
	Routed               bool       `json:"routed"`
	ArrivalDeadline      string     `json:"arrivalDeadline"`
	Events               []eventDTO `json:"events"`
	Legs                 []legDTO   `json:"legs"`
}

type locationDTO struct {
	UNLocode string `json:"locode"`
	Name     string `json:"name"`
}

type RouteCandidateDTO struct {
	Legs []legDTO `json:"legs"`
}

type legDTO struct {
	VoyageNumber string `json:"voyageNumber"`
	From         string `json:"from"`
	To           string `json:"to"`
	LoadTime     string `json:"loadTime"`
	UnloadTime   string `json:"unloadTime"`
}

type eventDTO struct {
	Description string `json:"description"`
	Expected    bool   `json:"expected"`
}

type BookingServiceFacade interface {
	BookNewCargo(origin, destination string, arrivalDeadline string) (string, error)
	LoadCargoForRouting(trackingId string) (cargoDTO, error)
	AssignCargoToRoute(trackingId string, candidate RouteCandidateDTO) error
	ChangeDestination(trackingId string, destinationUNLocode string) error
	RequestRoutesForCargo(trackingId string) []RouteCandidateDTO
	ListShippingLocations() []locationDTO
	ListAllCargos() []cargoDTO
}

type bookingServiceFacade struct {
	cargoRepository         cargo.CargoRepository
	locationRepository      location.LocationRepository
	handlingEventRepository cargo.HandlingEventRepository
	bookingService          application.BookingService
}

func (f *bookingServiceFacade) BookNewCargo(origin, destination string, arrivalDeadline string) (string, error) {
	millis, _ := strconv.ParseInt(fmt.Sprintf("%s", arrivalDeadline), 10, 64)
	trackingId, err := f.bookingService.BookNewCargo(location.UNLocode(origin), location.UNLocode(destination), time.Unix(millis/1000, 0))

	return string(trackingId), err
}

func (f *bookingServiceFacade) LoadCargoForRouting(trackingId string) (cargoDTO, error) {
	c, err := f.cargoRepository.Find(cargo.TrackingId(trackingId))

	if err != nil {
		return cargoDTO{}, err
	}

	return assemble(c, f.handlingEventRepository), nil
}

func (f *bookingServiceFacade) AssignCargoToRoute(trackingId string, candidate RouteCandidateDTO) error {
	legs := make([]cargo.Leg, 0)
	for _, l := range candidate.Legs {
		legs = append(legs, cargo.Leg{
			VoyageNumber:   voyage.VoyageNumber(l.VoyageNumber),
			LoadLocation:   location.UNLocode(l.From),
			UnloadLocation: location.UNLocode(l.To),
		})
	}

	return f.bookingService.AssignCargoToRoute(cargo.Itinerary{legs}, cargo.TrackingId(trackingId))
}

func (f *bookingServiceFacade) ChangeDestination(trackingId string, destinationUNLocode string) error {
	return f.bookingService.ChangeDestination(cargo.TrackingId(trackingId), location.UNLocode(destinationUNLocode))
}

func (f *bookingServiceFacade) RequestRoutesForCargo(trackingId string) []RouteCandidateDTO {
	itineraries := f.bookingService.RequestPossibleRoutesForCargo(cargo.TrackingId(trackingId))

	candidates := make([]RouteCandidateDTO, 0)
	for _, itin := range itineraries {
		legs := make([]legDTO, 0)
		for _, leg := range itin.Legs {
			legs = append(legs, legDTO{
				VoyageNumber: "S0001",
				From:         string(leg.LoadLocation),
				To:           string(leg.UnloadLocation),
				LoadTime:     "N/A",
				UnloadTime:   "N/A",
			})
		}
		candidates = append(candidates, RouteCandidateDTO{Legs: legs})
	}

	return candidates
}

func (f *bookingServiceFacade) ListShippingLocations() []locationDTO {
	locations := f.locationRepository.FindAll()

	dtos := make([]locationDTO, len(locations))
	for i, loc := range locations {
		dtos[i] = locationDTO{
			UNLocode: string(loc.UNLocode),
			Name:     loc.Name,
		}
	}

	return dtos
}

func (f *bookingServiceFacade) ListAllCargos() []cargoDTO {
	cargos := f.cargoRepository.FindAll()
	dtos := make([]cargoDTO, len(cargos))

	for i, c := range cargos {
		dtos[i] = assemble(c, f.handlingEventRepository)
	}

	return dtos
}

func NewBookingServiceFacade(cargoRepository cargo.CargoRepository, locationRepository location.LocationRepository, handlingEventRepository cargo.HandlingEventRepository, bookingService application.BookingService) BookingServiceFacade {
	return &bookingServiceFacade{cargoRepository, locationRepository, handlingEventRepository, bookingService}
}

func assemble(c cargo.Cargo, her cargo.HandlingEventRepository) cargoDTO {
	return cargoDTO{
		TrackingId:           string(c.TrackingId),
		Origin:               string(c.Origin),
		Destination:          string(c.RouteSpecification.Destination),
		ETA:                  c.Delivery.ETA.Format(time.RFC3339),
		NextExpectedActivity: "",
		Misrouted:            c.Delivery.RoutingStatus == cargo.Misrouted,
		Routed:               !c.Itinerary.IsEmpty(),
		ArrivalDeadline:      c.ArrivalDeadline.Format(time.RFC3339),
		StatusText:           assembleStatusText(c),
		Legs:                 assembleLegs(c),
		Events:               assembleEvents(c, her),
	}
}

func assembleStatusText(c cargo.Cargo) string {
	switch c.Delivery.TransportStatus {
	case cargo.NotReceived:
		return "Not received"
	case cargo.InPort:
		return fmt.Sprintf("In port %s", c.Delivery.LastKnownLocation)
	case cargo.OnboardCarrier:
		return fmt.Sprintf("Onboard voyage %s", c.Delivery.CurrentVoyage)
	case cargo.Claimed:
		return "Claimed"
	default:
		return "Unknown"
	}
}

func assembleLegs(c cargo.Cargo) []legDTO {
	legs := make([]legDTO, 0)
	for _, l := range c.Itinerary.Legs {
		legs = append(legs, legDTO{
			VoyageNumber: string(l.VoyageNumber),
			From:         string(l.LoadLocation),
			To:           string(l.UnloadLocation),
		})
	}
	return legs
}

func assembleEvents(c cargo.Cargo, r cargo.HandlingEventRepository) []eventDTO {
	h := r.QueryHandlingHistory(c.TrackingId)
	events := make([]eventDTO, len(h.HandlingEvents))
	for i, e := range h.HandlingEvents {
		var description string

		switch e.Activity.Type {
		case cargo.NotHandled:
			description = "Cargo has not yet been received."
		case cargo.Receive:
			description = fmt.Sprintf("Received in %s, at %s", e.Activity.Location, time.Now().Format(time.RFC3339))
		case cargo.Load:
			description = fmt.Sprintf("Loaded onto voyage %s in %s, at %s.", e.Activity.VoyageNumber, e.Activity.Location, time.Now().Format(time.RFC3339))
		case cargo.Unload:
			description = fmt.Sprintf("Unloaded off voyage %s in %s, at %s.", e.Activity.VoyageNumber, e.Activity.Location, time.Now().Format(time.RFC3339))
		case cargo.Claim:
			description = fmt.Sprintf("Claimed in %s, at %s.", e.Activity.Location, time.Now().Format(time.RFC3339))
		case cargo.Customs:
			description = fmt.Sprintf("Cleared customs in %s, at %s.", e.Activity.Location, time.Now().Format(time.RFC3339))
		default:
			description = "[Unknown status]"
		}

		events[i] = eventDTO{Description: description, Expected: c.Itinerary.IsExpected(e)}
	}

	return events
}