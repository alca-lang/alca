use std.gui;

html := """
<!doctype html>
<html lang="ru">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Змейка на Alca</title>
	<style>
		* { box-sizing: border-box; }
		body { margin: 0; min-height: 100vh; display: grid; place-items: center; background: #101820; color: #e8f1e8; font: 16px system-ui, sans-serif; }
		main { width: min(92vw, 520px); text-align: center; }
		h1 { margin: 0 0 8px; color: #9be564; }
		.bar { display: flex; justify-content: space-between; align-items: center; margin: 12px 2px; }
		#score { font-weight: 700; }
		button { border: 0; border-radius: 8px; padding: 9px 16px; background: #9be564; color: #15220e; font-weight: 700; cursor: pointer; }
		canvas { width: 100%; display: block; border: 2px solid #41633b; border-radius: 8px; background: #18252a; }
		p { color: #aab9ad; font-size: 14px; }
		.pad { display: grid; grid-template-columns: repeat(3, 54px); gap: 6px; justify-content: center; margin: 12px auto; }
		.pad button { padding: 10px; font-size: 18px; }
		.pad button:first-child { grid-column: 2; }
	</style>
</head>
<body>
	<main>
		<h1>Змейка</h1>
		<div class="bar"><span id="score">Счёт: 0</span><button id="restart">Начать заново</button></div>
		<canvas id="game" width="480" height="480" aria-label="Игра Змейка"></canvas>
		<p id="message">Управление: стрелки или WASD · пробел — пауза</p>
		<div class="pad" aria-label="Управление">
			<button data-dir="up" aria-label="Вверх">↑</button>
			<button data-dir="left" aria-label="Влево">←</button>
			<button data-dir="down" aria-label="Вниз">↓</button>
			<button data-dir="right" aria-label="Вправо">→</button>
		</div>
	</main>
	<script>
		(() => {
			const canvas = document.querySelector('#game');
			const ctx = canvas.getContext('2d');
			const scoreLabel = document.querySelector('#score');
			const message = document.querySelector('#message');
			const size = 24, cells = canvas.width / size;
			const directions = {
				up: { x: 0, y: -1 }, down: { x: 0, y: 1 },
				left: { x: -1, y: 0 }, right: { x: 1, y: 0 }
			};
			let snake, direction, nextDirection, food, score, timer, paused, over;

			function placeFood() {
				do {
					food = { x: Math.floor(Math.random() * cells), y: Math.floor(Math.random() * cells) };
				} while (snake.some(part => part.x === food.x && part.y === food.y));
			}

			function start() {
				clearInterval(timer);
				snake = [{ x: 10, y: 10 }, { x: 9, y: 10 }, { x: 8, y: 10 }];
				direction = { ...directions.right };
				nextDirection = { ...direction };
				score = 0; paused = false; over = false;
				scoreLabel.textContent = 'Счёт: 0';
				message.textContent = 'Управление: стрелки или WASD · пробел — пауза';
				placeFood(); draw();
				timer = setInterval(tick, 115);
			}

			function setDirection(name) {
				if (over || paused) return;
				const candidate = directions[name];
				if (candidate && (candidate.x !== -direction.x || candidate.y !== -direction.y)) nextDirection = { ...candidate };
			}

			function tick() {
				if (paused || over) return;
				direction = nextDirection;
				const head = { x: snake[0].x + direction.x, y: snake[0].y + direction.y };
				const eating = head.x === food.x && head.y === food.y;
				const body = eating ? snake : snake.slice(0, -1);
				if (head.x < 0 || head.y < 0 || head.x >= cells || head.y >= cells || body.some(part => part.x === head.x && part.y === head.y)) {
					over = true;
					message.textContent = `Игра окончена! Счёт: ${score}. Нажми «Начать заново».`;
					draw();
					return;
				}
				snake.unshift(head);
				if (eating) { score += 1; scoreLabel.textContent = `Счёт: ${score}`; placeFood(); }
				else snake.pop();
				draw();
			}

			function draw() {
				ctx.fillStyle = '#18252a'; ctx.fillRect(0, 0, canvas.width, canvas.height);
				ctx.strokeStyle = '#223238';
				for (let i = 0; i <= cells; i++) {
					ctx.beginPath(); ctx.moveTo(i * size, 0); ctx.lineTo(i * size, canvas.height); ctx.stroke();
					ctx.beginPath(); ctx.moveTo(0, i * size); ctx.lineTo(canvas.width, i * size); ctx.stroke();
				}
				ctx.fillStyle = '#ff6b6b';
				ctx.beginPath(); ctx.arc(food.x * size + size / 2, food.y * size + size / 2, size * .36, 0, Math.PI * 2); ctx.fill();
				snake.forEach((part, index) => {
					ctx.fillStyle = index === 0 ? '#b5f38b' : '#79c957';
					ctx.beginPath(); ctx.roundRect(part.x * size + 2, part.y * size + 2, size - 4, size - 4, 6); ctx.fill();
				});
				if (paused) {
					ctx.fillStyle = 'rgba(0,0,0,.55)'; ctx.fillRect(0, 0, canvas.width, canvas.height);
					ctx.fillStyle = '#fff'; ctx.font = 'bold 32px system-ui'; ctx.textAlign = 'center';
					ctx.fillText('Пауза', canvas.width / 2, canvas.height / 2);
				}
			}

			document.addEventListener('keydown', event => {
				const keys = { ArrowUp: 'up', w: 'up', W: 'up', ArrowDown: 'down', s: 'down', S: 'down', ArrowLeft: 'left', a: 'left', A: 'left', ArrowRight: 'right', d: 'right', D: 'right' };
				if (keys[event.key]) { event.preventDefault(); setDirection(keys[event.key]); }
				if (event.code === 'Space' && !over) {
					event.preventDefault(); paused = !paused;
					message.textContent = paused ? 'Пауза — нажми пробел, чтобы продолжить' : 'Управление: стрелки или WASD · пробел — пауза';
					draw();
				}
			});
			document.querySelectorAll('[data-dir]').forEach(button => button.addEventListener('click', () => setDirection(button.dataset.dir)));
			document.querySelector('#restart').addEventListener('click', start);
			start();
		})();
	</script>
</body>
</html>
""";

app := gui.webview("Змейка на Alca", html, 560, 720, {"engine": "webview2"});
app.wait();
