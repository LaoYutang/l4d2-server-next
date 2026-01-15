package controller

import (
	"fmt"
	"l4d2-manager-next/logic"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetPlugins(c *gin.Context) {
	plugins, err := logic.GetPlugins()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, plugins)
}

func UploadPlugin(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		if err = c.Request.ParseMultipartForm(32 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form: " + err.Error()})
			return
		}
		form = c.Request.MultipartForm
	}

	files := form.File["file"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	var errs []string
	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", header.Filename, err))
			continue
		}

		if err := logic.UploadPlugin(file, header.Size, header.Filename); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", header.Filename, err))
		}
		file.Close()
	}

	if len(errs) > 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": strings.Join(errs, "; ")})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Plugins uploaded successfully"})
}

func EnablePlugin(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}

	name := c.PostForm("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	if err := logic.EnablePlugin(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Plugin enabled successfully"})
}

func DisablePlugin(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	if err := logic.DisablePlugin(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Plugin disabled successfully"})
}

func DeletePlugin(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}

	name := c.PostForm("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	if err := logic.DeletePlugin(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Plugin deleted successfully"})
}
