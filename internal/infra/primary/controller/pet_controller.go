package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/core"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/config"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto"
	petin "github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/inputs/pets"
	petout "github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/outputs/pets"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/ginadapter"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"
)

// PetController exposes the pet records ("Ficha Mascota") of the authenticated
// account. Ownership is enforced by the use case: other accounts' pets are 404.
type PetController struct {
	authorization authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context]
	pets          core.PetUseCase
	r             *gin.Engine
	t             config.ITranslate
}

func NewPetController(
	authorization authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context],
	pets core.PetUseCase,
	r *gin.Engine,
	t config.ITranslate,
) *PetController {
	return &PetController{authorization: authorization, pets: pets, r: r, t: t}
}

func (pc *PetController) RunController() {
	if pc.pets == nil {
		return
	}
	group := pc.r.Group("/v1/pets")
	group.Use(ginadapter.Middleware(pc.authorization.AuthorizeJWT()))
	{
		group.GET("", pc.List())
		group.POST("", pc.Create())
		group.GET("/:id", pc.Get())
		group.PUT("/:id", pc.Update())
		group.DELETE("/:id", pc.Delete())
		group.GET("/:id/photo", pc.GetPhoto())
		group.PUT("/:id/photo", pc.PutPhoto())
		group.DELETE("/:id/photo", pc.DeletePhoto())
	}
}

// List godoc
// @Summary List my pets
// @Description Returns the pet records managed by the authenticated account, newest first.
// @Tags Pets
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.ResponseDto{data=[]petout.PetResponse}
// @Failure 401 {object} httpapi.Problem
// @Router /v1/pets [get]
func (pc *PetController) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(pc.authorization, c)
		if !ok {
			return
		}
		pets, err := pc.pets.List(c.Request.Context(), actor.GetId())
		if err != nil {
			writePetError(c, err)
			return
		}
		out := make([]petout.PetResponse, 0, len(pets))
		for _, pet := range pets {
			out = append(out, toPetResponse(pet))
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: out})
	}
}

// Create godoc
// @Summary Create a pet record
// @Description Creates a pet managed by the authenticated account. Public visibility keys default to the brand manual recommendations.
// @Tags Pets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body petin.PetRequest true "Pet record"
// @Success 201 {object} dto.ResponseDto{data=petout.PetResponse}
// @Failure 400 {object} httpapi.Problem
// @Failure 401 {object} httpapi.Problem
// @Failure 409 {object} httpapi.Problem
// @Failure 422 {object} httpapi.Problem
// @Router /v1/pets [post]
func (pc *PetController) Create() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(pc.authorization, c)
		if !ok {
			return
		}
		details, _, ok := bindPetRequest(c)
		if !ok {
			return
		}
		pet, err := pc.pets.Create(c.Request.Context(), actor.GetId(), details)
		if err != nil {
			writePetError(c, err)
			return
		}
		c.Header("Location", "/v1/pets/"+pet.ID)
		c.JSON(http.StatusCreated, dto.ResponseDto{Status: http.StatusCreated, Message: "Created", Data: toPetResponse(pet)})
	}
}

// Get godoc
// @Summary Get one of my pets
// @Tags Pets
// @Security BearerAuth
// @Produce json
// @Param id path string true "Pet ID"
// @Success 200 {object} dto.ResponseDto{data=petout.PetResponse}
// @Failure 401 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Router /v1/pets/{id} [get]
func (pc *PetController) Get() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(pc.authorization, c)
		if !ok {
			return
		}
		pet, err := pc.pets.Get(c.Request.Context(), actor.GetId(), c.Param("id"))
		if err != nil {
			writePetError(c, err)
			return
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: toPetResponse(pet)})
	}
}

// Update godoc
// @Summary Update one of my pets
// @Description Replaces the pet record. The version from the last read is required; a stale version returns 409.
// @Tags Pets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Pet ID"
// @Param body body petin.PetRequest true "Pet record with version"
// @Success 200 {object} dto.ResponseDto{data=petout.PetResponse}
// @Failure 400 {object} httpapi.Problem
// @Failure 401 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Failure 409 {object} httpapi.Problem
// @Failure 422 {object} httpapi.Problem
// @Router /v1/pets/{id} [put]
func (pc *PetController) Update() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(pc.authorization, c)
		if !ok {
			return
		}
		details, version, ok := bindPetRequest(c)
		if !ok {
			return
		}
		pet, err := pc.pets.Update(c.Request.Context(), actor.GetId(), c.Param("id"), version, details)
		if err != nil {
			writePetError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: toPetResponse(pet)})
	}
}

// Delete godoc
// @Summary Delete one of my pets
// @Description Deletes the pet record and its photo.
// @Tags Pets
// @Security BearerAuth
// @Param id path string true "Pet ID"
// @Success 204
// @Failure 401 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Router /v1/pets/{id} [delete]
func (pc *PetController) Delete() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(pc.authorization, c)
		if !ok {
			return
		}
		if err := pc.pets.Delete(c.Request.Context(), actor.GetId(), c.Param("id")); err != nil {
			writePetError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// GetPhoto godoc
// @Summary Get the photo of one of my pets
// @Tags Pets
// @Security BearerAuth
// @Produce image/jpeg
// @Produce image/png
// @Produce image/webp
// @Param id path string true "Pet ID"
// @Success 200 {file} binary
// @Failure 401 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Router /v1/pets/{id}/photo [get]
func (pc *PetController) GetPhoto() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(pc.authorization, c)
		if !ok {
			return
		}
		photo, err := pc.pets.GetPhoto(c.Request.Context(), actor.GetId(), c.Param("id"))
		if err != nil {
			writePetError(c, err)
			return
		}
		// Clients address photos with a ?v= version, so a private cache is safe.
		c.Header("Cache-Control", "private, max-age=86400")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Data(http.StatusOK, photo.ContentType, photo.Data)
	}
}

// PutPhoto godoc
// @Summary Upload or replace the photo of one of my pets
// @Description Raw image body (JPEG, PNG or WebP) up to 1 MiB. The type is detected from the bytes.
// @Tags Pets
// @Security BearerAuth
// @Accept image/jpeg
// @Accept image/png
// @Accept image/webp
// @Produce json
// @Param id path string true "Pet ID"
// @Success 200 {object} dto.ResponseDto{data=petout.PetResponse}
// @Failure 401 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Failure 413 {object} httpapi.Problem
// @Failure 422 {object} httpapi.Problem
// @Router /v1/pets/{id}/photo [put]
func (pc *PetController) PutPhoto() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(pc.authorization, c)
		if !ok {
			return
		}
		if c.Request.Body == nil {
			writePetError(c, fmt.Errorf("%w: empty image", domain.ErrInvalidPetPhoto))
			return
		}
		body := http.MaxBytesReader(c.Writer, c.Request.Body, domain.MaxPetPhotoBytes)
		defer body.Close()
		data, err := io.ReadAll(body)
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				httpapi.WriteProblem(c, httpapi.PayloadTooLarge(httpapi.RequestID(c)))
				return
			}
			writePetError(c, fmt.Errorf("%w: unreadable image", domain.ErrInvalidPetPhoto))
			return
		}
		pet, err := pc.pets.PutPhoto(c.Request.Context(), actor.GetId(), c.Param("id"), data)
		if err != nil {
			writePetError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: toPetResponse(pet)})
	}
}

// DeletePhoto godoc
// @Summary Remove the photo of one of my pets
// @Tags Pets
// @Security BearerAuth
// @Produce json
// @Param id path string true "Pet ID"
// @Success 200 {object} dto.ResponseDto{data=petout.PetResponse}
// @Failure 401 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Router /v1/pets/{id}/photo [delete]
func (pc *PetController) DeletePhoto() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(pc.authorization, c)
		if !ok {
			return
		}
		pet, err := pc.pets.DeletePhoto(c.Request.Context(), actor.GetId(), c.Param("id"))
		if err != nil {
			writePetError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: toPetResponse(pet)})
	}
}

// bindPetRequest decodes the JSON body into domain details. On failure the
// problem response is already written and ok is false.
func bindPetRequest(c *gin.Context) (domain.PetDetails, int64, bool) {
	var req petin.PetRequest
	if !httpapi.BindAndWrite(c, &req) {
		return domain.PetDetails{}, 0, false
	}
	birthDate, err := domain.ParsePetBirthDate(req.BirthDate)
	if err != nil {
		httpapi.WriteProblem(c, httpapi.Validation("Pet contains invalid fields", httpapi.RequestID(c),
			[]httpapi.FieldError{{Field: "birthDate", Code: "date"}}))
		return domain.PetDetails{}, 0, false
	}
	visibility := make(map[domain.PetPublicField]domain.PetVisibility, len(req.PublicVisibility))
	for field, value := range req.PublicVisibility {
		visibility[domain.PetPublicField(field)] = domain.PetVisibility(value)
	}
	details := domain.PetDetails{
		Name:                req.Name,
		Species:             domain.PetSpecies(req.Species),
		Breed:               req.Breed,
		Color:               req.Color,
		Sex:                 domain.PetSex(req.Sex),
		BirthDate:           birthDate,
		ChipNumber:          req.ChipNumber,
		WeightKg:            req.WeightKg,
		Size:                domain.PetSize(req.Size),
		DistinctiveFeatures: req.DistinctiveFeatures,
		Health: domain.PetHealth{
			Allergies:         req.Health.Allergies,
			Surgeries:         req.Health.Surgeries,
			SpecialConditions: req.Health.SpecialConditions,
			Temperament:       req.Health.Temperament,
		},
		Owner: domain.PetOwnerContact{
			FullName: req.Owner.FullName,
			Address:  req.Owner.Address,
			Comuna:   req.Owner.Comuna,
			Phone:    req.Owner.Phone,
			PhoneAlt: req.Owner.PhoneAlt,
		},
		PublicVisibility: visibility,
	}
	if req.SecondOwner != nil {
		details.SecondOwner = domain.PetSecondOwner{FullName: req.SecondOwner.FullName, Phone: req.SecondOwner.Phone}
	}
	return details, req.Version, true
}

func writePetError(c *gin.Context, err error) {
	requestID := httpapi.RequestID(c)
	var invalid *domain.PetValidationError
	switch {
	case errors.As(err, &invalid):
		fields := make([]httpapi.FieldError, 0, len(invalid.Issues))
		for _, issue := range invalid.Issues {
			fields = append(fields, httpapi.FieldError{Field: issue.Field, Code: issue.Code})
		}
		httpapi.WriteProblem(c, httpapi.Validation("Pet contains invalid fields", requestID, fields))
	case errors.Is(err, domain.ErrInvalidPetPhoto):
		httpapi.WriteProblem(c, httpapi.Validation("The photo must be a JPEG, PNG or WebP image up to 1 MiB", requestID,
			[]httpapi.FieldError{{Field: "photo", Code: "image"}}))
	case errors.Is(err, domain.ErrDuplicateChip):
		httpapi.WriteProblem(c, httpapi.Conflict("The chip number is already registered", requestID))
	case errors.Is(err, domain.ErrPetLimitReached):
		httpapi.WriteProblem(c, httpapi.Conflict("The pet limit for this account was reached", requestID))
	case errors.Is(err, domain.ErrStaleVersion):
		httpapi.WriteProblem(c, httpapi.Conflict("The pet was updated by someone else", requestID))
	default:
		httpapi.WriteProblem(c, httpapi.MapError(err, requestID))
	}
}

func toPetResponse(pet domain.Pet) petout.PetResponse {
	visibility := make(map[string]string, len(pet.PublicVisibility))
	for field, value := range pet.PublicVisibility {
		visibility[string(field)] = string(value)
	}
	photoURL := ""
	if pet.HasPhoto {
		photoURL = fmt.Sprintf("/v1/pets/%s/photo?v=%d", pet.ID, pet.PhotoUpdatedAt.UnixMilli())
	}
	return petout.PetResponse{
		ID:                  pet.ID,
		Name:                pet.Name,
		Species:             string(pet.Species),
		Breed:               pet.Breed,
		Color:               pet.Color,
		Sex:                 string(pet.Sex),
		BirthDate:           domain.FormatPetBirthDate(pet.BirthDate),
		ChipNumber:          pet.ChipNumber,
		WeightKg:            pet.WeightKg,
		Size:                string(pet.Size),
		DistinctiveFeatures: pet.DistinctiveFeatures,
		Health: petout.PetHealthResponse{
			Allergies:         nonNilStrings(pet.Health.Allergies),
			Surgeries:         nonNilStrings(pet.Health.Surgeries),
			SpecialConditions: nonNilStrings(pet.Health.SpecialConditions),
			Temperament:       pet.Health.Temperament,
		},
		Owner: petout.PetOwnerResponse{
			FullName: pet.Owner.FullName,
			Address:  pet.Owner.Address,
			Comuna:   pet.Owner.Comuna,
			Phone:    pet.Owner.Phone,
			PhoneAlt: pet.Owner.PhoneAlt,
		},
		SecondOwner:      petout.PetSecondOwnerResponse{FullName: pet.SecondOwner.FullName, Phone: pet.SecondOwner.Phone},
		PublicVisibility: visibility,
		PhotoURL:         photoURL,
		Version:          pet.Version,
		CreatedAt:        formatPetTime(pet.CreatedAt),
		UpdatedAt:        formatPetTime(pet.UpdatedAt),
	}
}

func formatPetTime(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339)
}

func nonNilStrings(items []string) []string {
	return append([]string{}, items...)
}
