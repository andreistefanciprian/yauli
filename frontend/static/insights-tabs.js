document.addEventListener("DOMContentLoaded", () => {
  const activeCategory = document.querySelector(".insights-category-pill.active");
  activeCategory?.scrollIntoView({ block: "nearest", inline: "center" });
});
