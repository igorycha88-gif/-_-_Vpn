export function initMobileMenu(): void {
  const burger = document.querySelector(".header__burger");
  const mobileMenu = document.querySelector(".mobile-menu");

  if (!burger || !mobileMenu) return;

  burger.addEventListener("click", () => {
    mobileMenu.classList.toggle("active");
    const spans = burger.querySelectorAll("span");
    const isOpen = mobileMenu.classList.contains("active");
    if (spans[0]) (spans[0] as HTMLElement).style.transform = isOpen ? "rotate(45deg) translate(5px, 5px)" : "";
    if (spans[1]) (spans[1] as HTMLElement).style.opacity = isOpen ? "0" : "1";
    if (spans[2]) (spans[2] as HTMLElement).style.transform = isOpen ? "rotate(-45deg) translate(5px, -5px)" : "";
  });

  mobileMenu.querySelectorAll("a").forEach((link) => {
    link.addEventListener("click", () => {
      mobileMenu.classList.remove("active");
      const spans = burger.querySelectorAll("span");
      if (spans[0]) (spans[0] as HTMLElement).style.transform = "";
      if (spans[1]) (spans[1] as HTMLElement).style.opacity = "1";
      if (spans[2]) (spans[2] as HTMLElement).style.transform = "";
    });
  });
}

document.addEventListener("DOMContentLoaded", initMobileMenu);
