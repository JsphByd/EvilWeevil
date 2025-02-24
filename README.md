
![nut-weevil-closeup-photography-brown-washington-dc-wallpaper](https://github.com/user-attachments/assets/d98afca8-6ed3-4dd5-b8ef-c041f36ef43e)

This script crawls through a domain to find search terms.

Still fairly basic.

Define domains and search terms in the .yaml file (note that subdomains must be defined as their own domains).
Run using ./src 

- External links are not parsed
- Subdomains not parsed unless defined as their own domains
- Script only parses CSS, JS, and HTML files linked within the same domain
- Script does not download any files locally
- Script does not respect robots.txt
- Script ignores mailto, fax, sms, geo, and skype hrefs
- Script currently excludes relative links containing ".."
- Script does not space out requests by default, define seconds to wait between requests using -w
- Use output pipe to save output to file (example: ./src > output.txt)
