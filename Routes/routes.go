package routes

import (
	"brandonplank.org/checkout/models"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	csv "github.com/gocarina/gocsv"
	"github.com/gofiber/fiber/v2"
	emailClient "github.com/jordan-wright/email"
	"io/ioutil"
	"log"
	"net/smtp"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

var MainGlobal = new(models.Main)

const DatabaseFile = "Storage/database.json"

var mutex sync.Mutex

func WriteJSONToFile() {
	database, err := os.OpenFile(DatabaseFile, os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	data, err := json.MarshalIndent(MainGlobal, "", "\t")

	err = ioutil.WriteFile(DatabaseFile, data, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
}

func SanitizeString(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), " ", "")
}

func IsAdmin(email string) bool {
	for _, school := range MainGlobal.Schools {
		if SanitizeString(email) == SanitizeString(school.AdminEmail) {
			return true
		}
	}
	return false
}

func ReadJSONToStruct() {
	content, _ := ioutil.ReadFile(DatabaseFile)
	if len(content) <= 1 {
		mainModel, _ := json.Marshal(models.Main{})
		err := ioutil.WriteFile(DatabaseFile, mainModel, os.ModePerm)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		err := json.Unmarshal(content, &MainGlobal)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func ReverseSlice(data interface{}) {
	value := reflect.ValueOf(data)
	if value.Kind() != reflect.Slice {
		panic(errors.New("data must be a slice type"))
	}
	valueLen := value.Len()
	if valueLen < 1 {
		return
	}
	for i := 0; i <= (valueLen-1)/2; i++ {
		reverseIndex := valueLen - 1 - i
		tmp := value.Index(reverseIndex).Interface()
		value.Index(reverseIndex).Set(value.Index(i))
		value.Index(i).Set(reflect.ValueOf(tmp))
	}
}

func IsStudentOut(name string, students []models.Student) bool {
	if students == nil {
		return false
	}
	for _, stu := range students {
		if stu.Name == name {
			if stu.SignIn == "Signed Out" {
				return true
			}
		}
	}
	return false
}

func Home(ctx *fiber.Ctx) error {
	email := ctx.Locals("email")
	logoURL := "assets/img/viking_logo.png"
	for _, school := range MainGlobal.Schools {
		for _, classroom := range school.Classrooms {
			if SanitizeString(classroom.Email) == SanitizeString(email.(string)) {
				if len(school.Logo) > 0 {
					logoURL = school.Logo
					break
				}
			}
		}
	}

	if IsAdmin(email.(string)) {
		return ctx.Render("admin", fiber.Map{
			"year": time.Now().Format("2006"),
			"logo": logoURL,
		})
	}

	return ctx.Render("main", fiber.Map{
		"year": time.Now().Format("2006"),
		"logo": logoURL,
	})
}

func Id(ctx *fiber.Ctx) error {
	email := ctx.Locals("email")

	nameBase64 := ctx.Params("name")
	nameData, err := base64.URLEncoding.DecodeString(nameBase64)
	if err != nil {
		return ctx.SendStatus(fiber.StatusBadRequest)
	}

	studentName := string(nameData)

	for schoolIndex, school := range MainGlobal.Schools {
		for classroomIndex, classroom := range school.Classrooms {
			if SanitizeString(classroom.Email) == SanitizeString(email.(string)) {
				if IsStudentOut(studentName, classroom.Students) {
					log.Println(fmt.Sprintf("%s has retured to %s's classroom", studentName, classroom.Name))
					var tempStudents []models.Student
					for _, stu := range classroom.Students {
						if stu.Name == studentName {
							if stu.SignIn == "Signed Out" {
								stu.SignIn = time.Now().Format("3:04 pm")
							}
						}
						tempStudents = append(tempStudents, stu)
					}
					mutex.Lock()
					MainGlobal.Schools[schoolIndex].Classrooms[classroomIndex].Students = tempStudents
					mutex.Unlock()
				} else {
					log.Println(fmt.Sprintf("%s has left from %s's classroom", studentName, classroom.Name))
					mutex.Lock()
					MainGlobal.Schools[schoolIndex].Classrooms[classroomIndex].Students = append(classroom.Students, models.Student{Name: studentName, SignOut: time.Now().Format("3:04 pm"), SignIn: "Signed Out", Date: time.Now().Format("01/02/2006"), Classroom: classroom.Name})
					mutex.Unlock()
				}
				WriteJSONToFile()
				return ctx.SendStatus(fiber.StatusOK)
			}
		}
	}
	return ctx.SendStatus(fiber.StatusBadRequest)
}

func GetCSV(ctx *fiber.Ctx) error {
	email := ctx.Locals("email")
	for _, school := range MainGlobal.Schools {
		if len(school.Classrooms) > 0 {
			for _, classroom := range school.Classrooms {
				if SanitizeString(classroom.Email) == SanitizeString(email.(string)) {
					if len(classroom.Students) < 1 {
						return ctx.SendString("No students yet")
					}
					var students models.PublicStudents
					students = models.StudentsToPublicStudents(classroom.Students)
					sort.Sort(students)
					ReverseSlice(students)
					content, _ := csv.MarshalBytes(students)
					return ctx.Send(content)
				}
			}
		}
	}
	return ctx.SendStatus(fiber.StatusInternalServerError)
}

func GetAdminCSV(ctx *fiber.Ctx) error {
	for _, school := range MainGlobal.Schools {
		if len(school.Classrooms) > 0 {
			var allStudents models.Students
			for _, classroom := range school.Classrooms {
				if len(classroom.Students) < 1 {
					continue
				}
				for _, student := range classroom.Students {
					allStudents = append(allStudents, student)
				}
			}
			sort.Sort(allStudents)
			ReverseSlice(allStudents)
			content, _ := csv.MarshalBytes(allStudents)
			return ctx.Send(content)
		}
	}
	return ctx.SendStatus(fiber.StatusInternalServerError)
}

func AdminSearchStudent(ctx *fiber.Ctx) error {
	nameBase64 := ctx.Params("name")
	nameData, err := base64.URLEncoding.DecodeString(nameBase64)
	if err != nil {
		return ctx.SendStatus(fiber.StatusBadRequest)
	}
	studentName := string(nameData)

	for _, school := range MainGlobal.Schools {
		if len(school.Classrooms) > 0 {
			var allStudents models.Students
			for _, classroom := range school.Classrooms {
				if len(classroom.Students) < 1 {
					continue
				}
				for _, student := range classroom.Students {
					if strings.Contains(SanitizeString(student.Name), SanitizeString(studentName)) {
						allStudents = append(allStudents, student)
					}
				}
			}
			sort.Sort(allStudents)
			ReverseSlice(allStudents)
			content, _ := csv.MarshalBytes(allStudents)
			return ctx.Send(content)
		}
	}
	return ctx.SendStatus(fiber.StatusInternalServerError)
}

func CSVFile(ctx *fiber.Ctx) error {
	email := ctx.Locals("email")
	for _, school := range MainGlobal.Schools {
		for _, classroom := range school.Classrooms {
			if SanitizeString(classroom.Email) == SanitizeString(email.(string)) {
				var students models.PublicStudents
				students = models.StudentsToPublicStudents(classroom.Students)
				sort.Sort(students)
				studentsBytes, err := csv.MarshalBytes(students)
				if err != nil {
					return ctx.SendStatus(fiber.StatusBadRequest)
				}
				ctx.Append("Content-Disposition", "attachment; filename=\"classroom.csv\"")
				ctx.Append("Content-Type", "text/csv")
				return ctx.Send(studentsBytes)
			}
		}
	}
	return ctx.SendStatus(fiber.StatusBadRequest)
}

func AdminCSVFile(ctx *fiber.Ctx) error {
	for _, school := range MainGlobal.Schools {
		if len(school.Classrooms) > 0 {
			var allStudents models.Students
			for _, classroom := range school.Classrooms {
				if len(classroom.Students) < 1 {
					continue
				}
				for _, student := range classroom.Students {
					allStudents = append(allStudents, student)
				}
			}
			sort.Sort(allStudents)
			ReverseSlice(allStudents)
			content, _ := csv.MarshalBytes(allStudents)
			ctx.Append("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.csv\"", school.Name))
			ctx.Append("Content-Type", "text/csv")
			return ctx.Send(content)
		}
	}
	return ctx.SendStatus(fiber.StatusBadRequest)
}

func IsOut(ctx *fiber.Ctx) error {
	nameBase64 := ctx.Params("name")
	nameData, err := base64.URLEncoding.DecodeString(nameBase64)
	if err != nil {
		return ctx.SendStatus(fiber.StatusBadRequest)
	}
	studentName := string(nameData)

	email := ctx.Locals("email")
	for _, school := range MainGlobal.Schools {
		for _, classroom := range school.Classrooms {
			if SanitizeString(classroom.Email) == SanitizeString(email.(string)) {
				type out struct {
					IsOut bool   `json:"isOut"`
					Name  string `json:"name"`
				}
				return ctx.JSON(out{IsOut: IsStudentOut(studentName, classroom.Students), Name: studentName})
			}
		}
	}
	return ctx.SendStatus(fiber.StatusBadRequest)
}

func CleanClass(ctx *fiber.Ctx) error {
	var classroomEmail string
	email := ctx.Locals("email")
	emailBase64 := ctx.Params("email")
	if len(emailBase64) > 0 {
		emailData, err := base64.URLEncoding.DecodeString(emailBase64)
		if err != nil {
			return ctx.SendStatus(fiber.StatusBadRequest)
		}
		classroomEmail = string(emailData)
	}

	if len(classroomEmail) < 1 {
		classroomEmail = email.(string)
	}

	for schoolsIndex, school := range MainGlobal.Schools {
		for classroomsIndex, classroom := range school.Classrooms {
			if SanitizeString(classroom.Email) == SanitizeString(email.(string)) {
				mutex.Lock()
				MainGlobal.Schools[schoolsIndex].Classrooms[classroomsIndex].Students = models.Students{}
				mutex.Unlock()
				WriteJSONToFile()
				return ctx.SendStatus(fiber.StatusOK)
			}
		}
	}
	return ctx.SendStatus(fiber.StatusNotFound)
}

func DoesSchoolHaveStudents(classes []models.Classroom) bool {
	for _, class := range classes {
		if len(class.Students) > 0 {
			return true
		}
	}
	return false
}

func CleanStudents() {
	for schoolsIndex, school := range MainGlobal.Schools {
		for classroomsIndex := range school.Classrooms {
			MainGlobal.Schools[schoolsIndex].Classrooms[classroomsIndex].Students = models.Students{}
		}
	}
}

func TeacherHasAdmin(email string) bool {
	for _, school := range MainGlobal.Schools {
		for _, classroom := range school.Classrooms {
			if SanitizeString(email) == SanitizeString(classroom.Email) {
				return classroom.IsAdmin
			}
		}
	}
	return false
}

func AddTeacher(ctx *fiber.Ctx) error {
	email := ctx.Locals("email")
	payloadP := new(map[string]interface{})
	err := ctx.BodyParser(&payloadP)
	if err != nil {
		return err
	}
	payload := *payloadP
	TeacherName := payload["name"]
	TeacherEmail := payload["email"]
	for schoolIndex, school := range MainGlobal.Schools {
		if SanitizeString(school.AdminEmail) == SanitizeString(email.(string)) || TeacherHasAdmin(email.(string)) {
			MainGlobal.Schools[schoolIndex].Classrooms = append(MainGlobal.Schools[schoolIndex].Classrooms, models.Classroom{Name: TeacherName.(string), Email: SanitizeString(TeacherEmail.(string)), Password: "govikings2022", Students: models.Students{}})
			WriteJSONToFile()
			return ctx.SendStatus(fiber.StatusOK)
		}
	}
	return ctx.SendStatus(fiber.StatusBadRequest)
}

func remove(slice []models.Classroom, s int) []models.Classroom {
	return append(slice[:s], slice[s+1:]...)
}

func RemoveTeacher(ctx *fiber.Ctx) error {
	email := ctx.Locals("email")
	payloadP := new(map[string]interface{})
	err := ctx.BodyParser(payloadP)
	if err != nil {
		return err
	}
	payload := *payloadP
	TeacherEmail := payload["email"]
	log.Println("Removing", TeacherEmail)
	for schoolIndex, school := range MainGlobal.Schools {
		if SanitizeString(school.AdminName) == SanitizeString(email.(string)) || TeacherHasAdmin(email.(string)) {
			for classroomIndex, classroom := range school.Classrooms {
				if SanitizeString(TeacherEmail.(string)) == SanitizeString(classroom.Email) {
					MainGlobal.Schools[schoolIndex].Classrooms = remove(MainGlobal.Schools[schoolIndex].Classrooms, classroomIndex)
					WriteJSONToFile()
					return ctx.SendStatus(fiber.StatusOK)
				}
			}
		}
	}
	return ctx.SendStatus(fiber.StatusBadRequest)
}

func ChangePassword(ctx *fiber.Ctx) error {
	email := ctx.Locals("email")

	payload := struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}{}
	err := ctx.BodyParser(&payload)
	if err != nil {
		return err
	}

	for schoolIndex, school := range MainGlobal.Schools {
		for classroomIndex, classroom := range school.Classrooms {
			if SanitizeString(email.(string)) == SanitizeString(classroom.Email) {
				if payload.CurrentPassword != MainGlobal.Schools[schoolIndex].Classrooms[classroomIndex].Password {
					return ctx.SendStatus(fiber.StatusUnauthorized)
				}
				MainGlobal.Schools[schoolIndex].Classrooms[classroomIndex].Password = payload.NewPassword
				WriteJSONToFile()
				return ctx.SendStatus(fiber.StatusOK)
			}
		}
	}
	return ctx.SendStatus(fiber.StatusBadRequest)
}

func DailyRoutine() {
	pass := os.Getenv("PASSWORD")

	studentsFile, _ := os.OpenFile(DatabaseFile, os.O_RDWR|os.O_CREATE, os.ModePerm)
	defer studentsFile.Close()

	for _, school := range MainGlobal.Schools {
		if len(school.AdminEmail) < 1 || len(school.AdminName) < 1 || len(school.AdminPassword) < 1 {
			continue
		}
		if DoesSchoolHaveStudents(school.Classrooms) {
			var allStudents models.Students
			for _, classroom := range school.Classrooms {
				if len(classroom.Students) < 1 {
					continue
				}
				for _, student := range classroom.Students {
					allStudents = append(allStudents, student)
				}
			}
			sort.Sort(allStudents)
			ReverseSlice(allStudents)
			content, _ := csv.MarshalBytes(allStudents)
			csvReader := bytes.NewReader(content)

			schoolEmail := emailClient.NewEmail()
			schoolEmail.From = "Classroom Attendance <planksprojects@gmail.com>"
			schoolEmail.Subject = "Classroom Sign-Outs"
			schoolEmail.To = []string{school.AdminEmail}
			schoolEmail.Text = []byte("This is an automated email to " + school.Name)
			schoolEmail.Attach(csvReader, fmt.Sprintf("%s.csv", school.Name), "text/csv")
			err := schoolEmail.Send("smtp.gmail.com:587", smtp.PlainAuth("", "planksprojects@gmail.com", pass, "smtp.gmail.com"))
			if err != nil {
				log.Println(err)
			}
		}
	}

	for _, school := range MainGlobal.Schools {
		for _, class := range school.Classrooms {
			students := class.Students
			if len(students) < 1 {
				continue
			}
			csvClass, err := csv.MarshalBytes(students)
			if err != nil {
				log.Println(err)
			}
			if len(csvClass) < 5 {
				continue
			}
			csvReader := bytes.NewReader(csvClass)
			classroomEmail := emailClient.NewEmail()
			classroomEmail.From = "Classroom Attendance <planksprojects@gmail.com>"
			classroomEmail.Subject = "Classroom Sign-Outs"
			classroomEmail.To = []string{class.Email}
			classroomEmail.Text = []byte("This is an automated email to " + class.Name)
			classroomEmail.Attach(csvReader, fmt.Sprintf("%s.csv", class.Name), "text/csv")
			err = classroomEmail.Send("smtp.gmail.com:587", smtp.PlainAuth("", "planksprojects@gmail.com", pass, "smtp.gmail.com"))
			if err != nil {
				log.Println(err)
			}
		}
	}
	WriteJSONToFile()
}
