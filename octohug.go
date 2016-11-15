//
// octohug
//
// copies octopress posts to hugo posts
//   converts the header
//   converts categories and tags to hugo format in header
//   if run in the octopress directory, replaces include_file with the contents
//
// http://codebrane.com/blog/2015/09/10/migrating-from-octopress-to-hugo/

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var octopressPostsDirectory string
var hugoPostDirectory string

func readFile(path string) (string, error) {
	file, fileError := os.Open(path)
	if fileError != nil {
	}
	defer file.Close()
	var buffer []byte
	fileReader := bufio.NewReaderSize(file, 10*1024)
	line, isPrefix, lineError := fileReader.ReadLine()
	for lineError == nil && !isPrefix {
		buffer = append(buffer, line...)
		buffer = append(buffer, byte('\n'))
		line, isPrefix, lineError = fileReader.ReadLine()
	}
	if isPrefix {
		fmt.Fprintln(os.Stderr, "buffer size too small")
		return "", nil
	}

	return string(buffer), nil
}

func visit(path string, fileInfo os.FileInfo, err error) error {
	if fileInfo.IsDir() {
		return nil
	}

	// Get the base filename of the post
	octopressFilename := filepath.Base(path)

	// Need to strip off the initial date and final .markdown from the post filename
	regex := regexp.MustCompile(`(^\d{4}-\d{2}-\d{2})-(.*).(markdown|md|html)`)
	matches := regex.FindStringSubmatch(octopressFilename)

	// Ignore non-matching filenames (i.e. do no dereference nil)
	if matches == nil {
		return nil
	}
	octopressFilenameWithoutExtension := matches[2]
	hugoFilename := hugoPostDirectory + "/" + matches[1] + "-" + octopressFilenameWithoutExtension + ".md"
	fmt.Printf("%s\n%s\n", path, hugoFilename)

	// Open the octopress file
	octopressFile, octopressFileError := os.Open(path)
	// Nothing to do if we can open the source file
	if octopressFileError != nil {
		fmt.Fprintf(os.Stderr, "Error opening octopress file %s, ignoring\n", path)
		return nil
	}
	defer octopressFile.Close()

	// Create the hugo file
	hugoFile, hugoFileError := os.Create(hugoFilename)
	if hugoFileError != nil {
		fmt.Fprintf(os.Stderr, "could not create hugo file: %v\n", hugoFileError)
		return nil
	}
	defer hugoFile.Close()
	hugoFileWriter := bufio.NewWriter(hugoFile)

	// octopressDateRegex := regexp.MustCompile(`^date:`)
	octopressCategoryOrTagNameRegex := regexp.MustCompile(`^- (.*)`)

	// Read the octopress file line by line
	headerTagSeen := false
	inCategories := false
	firstCategoryAdded := false
	inTags := false
	inMeta := false
	firstTagAdded := false
	isDraft := false
	octopressFileReader := bufio.NewReaderSize(octopressFile, 10*1024)
	octopressLine, isPrefix, lineError := octopressFileReader.ReadLine()
	for lineError == nil && !isPrefix {
		octopressLineAsString := string(octopressLine)
		if octopressLineAsString == "---" {
			headerTagSeen = !headerTagSeen
			if inCategories || inTags {
				hugoFileWriter.WriteString("]\n")
				inCategories = false
				inTags = false
			}
			if isDraft {
				hugoFileWriter.WriteString("draft: true\n")
			}
			octopressLineAsString = string("+++")
		}
		if inMeta == true {
			if strings.HasPrefix(octopressLineAsString, "  ") || strings.HasPrefix(octopressLineAsString, "\t") {
				fmt.Println(octopressLineAsString)
				// nothing to do
			} else {
				inMeta = false
			}
		}
		if inMeta == true {
		} else if strings.Contains(octopressLineAsString, "type: post") || strings.Contains(octopressLineAsString, "status: publish") || strings.Contains(octopressLineAsString, "meta: {}") {
		} else if strings.Contains(octopressLineAsString, "status: private") {
			isDraft = true
		} else if strings.Contains(octopressLineAsString, "meta:") {
			inMeta = true
		} else if strings.Contains(octopressLineAsString, "categories:") || strings.Contains(octopressLineAsString, "category:") {
			inCategories = true
			hugoFileWriter.WriteString("categories = [")
		} else if strings.Contains(octopressLineAsString, "tags:") {
			if inCategories {
				inCategories = false
				hugoFileWriter.WriteString("]\n")
			}
			if strings.TrimSpace(octopressLineAsString) == "tags:" {
				inTags = true
				hugoFileWriter.WriteString("tags = [")
			} else {
				inTags = true
				regex := regexp.MustCompile(`\[(.*)\]`)
				matches := regex.FindStringSubmatch(octopressLineAsString)
				if len(matches) > 1 {
					fmt.Println(matches[1])
					str := matches[1]
					list := strings.Split(str, " ")
					if len(list) > 0 {
						data := "tags = ["
						for i, x := range list {
							data += fmt.Sprintf("\"%s\"", x)
							if i != len(list)-1 {
								data += ", "
							}
						}
						data += "]\n"
						hugoFileWriter.WriteString(data)
					} else {
						hugoFileWriter.WriteString("tags = []\n")
					}
				}
				inTags = false
			}
		} else if strings.Contains(octopressLineAsString, "keywords: ") {
			inCategories = false
			if inTags {
				hugoFileWriter.WriteString("]\n")
				inTags = false
			}
			hugoFileWriter.WriteString("keywords = [")
			parts := strings.Split(octopressLineAsString, ": ")
			keywords := strings.Split(strings.Replace(parts[1], "\"", "", -1), ",")
			firstKeyword := true
			for _, keyword := range keywords {
				if !firstKeyword {
					hugoFileWriter.WriteString(",")
				}
				hugoFileWriter.WriteString("\"" + keyword + "\"")
				firstKeyword = false
			}
			hugoFileWriter.WriteString("]\n")
		} else if inCategories && !inTags {
			matches = octopressCategoryOrTagNameRegex.FindStringSubmatch(octopressLineAsString)
			if len(matches) > 1 {
				if firstCategoryAdded {
					hugoFileWriter.WriteString(", ")
				}
				hugoFileWriter.WriteString("\"" + matches[1] + "\"")
				firstCategoryAdded = true
			}
		} else if octopressLineAsString == "tags:" {
			inTags = true
			hugoFileWriter.WriteString("Tags = [")
		} else if inTags {
			var tag = ""
			matches = octopressCategoryOrTagNameRegex.FindStringSubmatch(octopressLineAsString)
			if len(matches) > 1 {
				// fmt.Println("matches tag")
				if firstTagAdded {
					hugoFileWriter.WriteString(", ")
				}
				tag = strings.Replace(matches[1], "'", "", -1)
				tag = strings.Replace(tag, "\"", "", -1)
				hugoFileWriter.WriteString("\"" + tag + "\"")
				firstTagAdded = true
			}
		} else if strings.Contains(octopressLineAsString, "date: ") {
			parts := strings.Split(octopressLineAsString, " ")
			if len(parts[1]) == 10 && len(parts) == 3 {
				if len(parts[2]) == 5 {
					hugoFileWriter.WriteString("date = \"" + parts[1] + "T" + parts[2] + ":00+05:30" + "\"\n")
				} else {
					hugoFileWriter.WriteString("date = \"" + parts[1] + "T" + parts[2] + "+05:30" + "\"\n")
				}
			} else if len(parts[1]) == 10 && len(parts) == 4 {
				tz := parts[3]
				if tz == "+0530" {
					tz = "+05:30"
				}
				hugoFileWriter.WriteString("date = \"" + parts[1] + "T" + parts[2] + tz + "\"\n")
			} else {
				hugoFileWriter.WriteString("date = \"" + parts[1] + "\"\n")
			}
			octoSlugDate := strings.Replace(parts[1], "-", "/", -1)
			octoFriendlySlug := octoSlugDate + "/" + octopressFilenameWithoutExtension
			hugoFileWriter.WriteString("slug = \"" + octoFriendlySlug + "\"\n")
		} else if strings.Contains(octopressLineAsString, "title: ") {
			parts := strings.Split(octopressLineAsString, "title: ")
			title := strings.Trim(parts[1], "\"")
			title = strings.Trim(title, "'")
			title = strings.TrimSpace(title)
			if title == "" {
				title = octopressFilenameWithoutExtension
			}
			hugoFileWriter.WriteString(fmt.Sprintf("title = \"%s\"\n", title))
		} else if strings.Contains(octopressLineAsString, "description: ") {
			parts := strings.Split(octopressLineAsString, ": ")
			hugoFileWriter.WriteString("description = " + parts[1] + "\n")
		} else if strings.Contains(octopressLineAsString, "layout: ") {
		} else if strings.Contains(octopressLineAsString, "author: ") {
		} else if strings.Contains(octopressLineAsString, "comments: ") {
		} else if strings.Contains(octopressLineAsString, "slug: ") {
		} else if strings.Contains(octopressLineAsString, "wordpress_id: ") {
		} else if strings.Contains(octopressLineAsString, "published: ") {
			// hugoFileWriter.WriteString("published = false\n")
		} else if strings.Contains(octopressLineAsString, "include_code") {
			parts := strings.Split(octopressLineAsString, " ")
			// can be:
			// {% include_code [RedViewController.m] lang:objectivec slidernav/RedViewController.m %}
			// or
			// {% include_code [RedViewController.m] slidernav/RedViewController.m %}
			codeFilePath := "source/downloads/code/" + parts[len(parts)-2]
			codeFileContent, _ := readFile(codeFilePath)
			codeFileContent = strings.Replace(codeFileContent, "<", "&lt;", -1)
			codeFileContent = strings.Replace(codeFileContent, ">", "&gt;", -1)
			hugoFileWriter.WriteString("<pre><code>\n" + codeFileContent + "</code></pre>\n")
		} else {
			hugoFileWriter.WriteString(octopressLineAsString + "\n")
		} // if octopressLineAsString == "categories:"

		hugoFileWriter.Flush()
		octopressLine, isPrefix, lineError = octopressFileReader.ReadLine()
	}
	if isPrefix {
		fmt.Fprintln(os.Stderr, "buffer size too small")
	}
	return nil
}

func init() {
	flag.StringVar(&octopressPostsDirectory, "octo", "source/_posts", "path to octopress posts directory")
	flag.StringVar(&hugoPostDirectory, "hugo", "content/post", "path to hugo post directory")
}

func main() {
	flag.Parse()

	// Check that we can trust octopressPostsDirectory
	if _, err := os.Stat(octopressPostsDirectory); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading directory: %v\n", err)
		os.Exit(-1)
	}
	os.MkdirAll(hugoPostDirectory, 0777)
	filepath.Walk(octopressPostsDirectory, visit)
}
