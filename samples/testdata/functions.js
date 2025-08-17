/*
 * Function: functions
 * Behavior: Decompile SpiderMonkey bytecode into JavaScript
 */
function functions() {
    // main function
    function main() {}
    
    // createStyle function
    function createStyle() {
        var style = document.createElement("style");
        style.type = "text/css";
        var cssText = [
            ".cocosLoading{position:absolute;top:0;left:0;width:100%;height:100%;background:#252525}",
            ".cocosLoading .image{display:block;width:100%;height:85%;background:url(./res/icon.png) no-repeat center; max-width:1000px;background-size: 30% auto; margin:0 auto;animation: animate-scale 0.7s, animate-opacity 0.7s, animate-blur 0.7s, animage-glow 1.2s ease-in-out;}",
            ".cocosLoading ul{height:5%; width:100%;margin-left:50%;padding-inline-start:0px;transform:translateX(-90px);}",
            ".cocosLoading span{color:#f05a23;text-align:center;font-size:24px;display:block;width:100%;height:10%;background-size: 30% auto;position: absolute}",
            ".cocosLoading li{list-style:none;float:left;border-radius:24px;width:24px;height:24px;background:#FFF;margin:5px 0 0 10px}",
            ".cocosLoading li .ball,.cocosLoading li .unball{background-color:#f05a23;background-image:-moz-linear-gradient(90deg,#f05a23 25%,#f0b73c);background-image:-webkit-linear-gradient(90deg,#f05a23 25%,#f0b73c);width:24px;height:24px;border-radius:50px}",
            ".cocosLoading li .ball{transform:scale(0);-moz-transform:scale(0);-webkit-transform:scale(0);animation:showDot 1s linear forwards;-moz-animation:showDot 1s linear forwards;-webkit-animation:showDot 1s linear forwards}",
            ".cocosLoading li .unball{transform:scale(1);-moz-transform:scale(1);-webkit-transform:scale(1);animation:hideDot 1s linear forwards;-moz-animation:hideDot 1s linear forwards;-webkit-animation:hideDot 1s linear forwards}",
            "@keyframes showDot{0%{transform:scale(0,0)}100%{transform:scale(1,1)}}",
            "@-moz-keyframes showDot{0%{-moz-transform:scale(0,0)}100%{-moz-transform:scale(1,1)}}",
            "@-webkit-keyframes showDot{0%{-webkit-transform:scale(0,0)}100%{-webkit-transform:scale(1,1)}}",
            "@keyframes hideDot{0%{transform:scale(1,1)}100%{transform:scale(0,0)}}",
            "@-moz-keyframes hideDot{0%{-moz-transform:scale(1,1)}100%{-moz-transform:scale(0,0)}}",
            "@-webkit-keyframes hideDot{0%{-webkit-transform:scale(1,1)}100%{-webkit-transform:scale(0,0)}}",
            "@keyframes animate-scale{0% {transform:scale(0.2);} 25% {transform:scale(1.2);} 100% {transform:scale(1.0);}}",
            "@keyframes animate-opacity{0% {opacity: 0.2;} 100% {opacity: 1;}}",
            "@keyframes animate-blur {0%{-webkit-filter: blur(5px);filter: blur(5px);filter: drop-shadow(16px 16px 20px rgb(247, 148, 36))}100% {-webkit-filter: blur(0px);filter: blur(0px);filter: drop-shadow(0px 0px 0px (247, 148, 36))}}"
        ].join("");
        style.innerHTML = cssText;
        document.head.appendChild(style);
    }
    
    // createDom function
    function createDom(id) {
        var div1 = document.createElement("div");
        div1.id = id;
        div1.className = "cocosLoading";
        document.body.appendChild(div1);

        var ul = document.createElement("ul");
        ul.style.height = "5%";
        ul.style.width = "100%";
        ul.style.marginLeft = "50%";
        ul.style.paddingInlineStart = "0px";
        ul.style.transform = "translateX(-90px)";
        div1.appendChild(ul);
        
        var li = document.createElement("li");
        li.style.listStyle = "none";
        li.style.float = "left";
        li.style.borderRadius = "24px";
        li.style.width = "24px";
        li.style.height = "24px";
        li.style.background = "#FFF";
        li.style.margin = "5px 0 0 10px";
        ul.appendChild(li);

        var ball = document.createElement("div");
        ball.className = "ball";
        ball.style.transform = "scale(0)";
        li.appendChild(ball);
        
        var unball = document.createElement("div");
        unball.className = "unball";
        unball.style.transform = "scale(1)";
        li.appendChild(unball);

        div2 = document.createElement("div");
        div2.className = "image";
        div1.appendChild(div2);

        span = document.createElement("span");
        span.style.color = "#f05a23";
        span.style.textAlign = "center";
        span.style.fontSize = "24px";
        span.style.display = "block";
        span.style.width = "100%";
        span.style.height = "10%";
        span.style.backgroundSize = "30% auto";
        div1.appendChild(span);

        document.body.appendChild(div2);
    }
    
    // startAnimation function
    function startAnimation() {
        animation.index = 0;
        animation.direction = true;
        animation.time = 300;
        
        setTimeout(function() {
            if (animation.callback) {
                animation.callback();
            }
            
            while (true) {
                var ball = document.getElementsByClassName("ball")[animation.index];
                ball.className = "unball";
                
                animation.index++;
                if (document.getElementsByClassName("ball").length <= animation.index) {
                    animation.direction = false;
                    animation.time = 1000;
                } else if (animation.index >= document.getElementsByClassName("ball").length) {
                    animation.index = 0;
                    animation.time = 300;
                }
                
                setTimeout(function() {
                    var ball = document.getElementsByClassName("ball")[animation.index];
                    ball.className = "ball";
                    
                    animation.index++;
                    if (document.getElementsByClassName("ball").length <= animation.index) {
                        animation.direction = false;
                        animation.time = 1000;
                    } else if (animation.index >= document.getElementsByClassName("ball").length) {
                        animation.index = 0;
                        animation.time = 300;
                    }
                    
                    startAnimation.animation();
                }, animation.time);
            }
        }, animation.time);
    }

    // unknown function
    function unknown() {
        var style = document.createElement("style");
        style.type = "text/css";
        
        var cssText = [
            "#cocosLoading{background-color:#000}"
        ].join("");
        
        style.innerHTML = cssText;
        document.body.style.background = "#000";
        document.head.appendChild(style);
    }
}