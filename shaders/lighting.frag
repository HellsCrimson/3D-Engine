#version 460 core

in vec3 FragPos;
in vec3 Normal;
in vec2 TexCoords;

layout(location = 0) out vec4 colorOut;

struct Material {
    sampler2D texture_diffuse;
    sampler2D texture_specular;
    sampler2D emission;
    float shininess;

    bool has_diffuse;
    bool has_specular;
    bool has_emission;
    bool has_reflection;

    sampler2D missing_texture;
};

struct DirLight {
    vec3 direction;

    vec3 ambient;
    vec3 diffuse;
    vec3 specular;
};

struct PointLight {
    vec3 position;

    float constant;
    float linear;
    float quadratic;

    vec3 ambient;
    vec3 diffuse;
    vec3 specular;
};

struct SpotLight {
    vec3 position;
    vec3 direction;
    float cutOff;
    float outerCutOff;

    float constant;
    float linear;
    float quadratic;

    vec3 ambient;
    vec3 diffuse;
    vec3 specular;

    bool isEnabled;
};

uniform vec3 viewPos;
uniform Material material;

uniform samplerCube skybox;

#define MAX_POINT_LIGHT 4
uniform int nb_point_light;
uniform DirLight dirLight;
uniform PointLight pointLights[MAX_POINT_LIGHT];
uniform SpotLight spotLight;

vec3 CalcDirLight(DirLight light, vec3 normal, vec3 viewDir);
vec3 CalcPointLight(PointLight light, vec3 normal, vec3 fragPos, vec3 viewDir);
vec3 CalcSpotLight(SpotLight light, vec3 normal, vec3 fragPos, vec3 viewDir);

void main()
{
    vec3 norm = normalize(Normal);
    vec3 viewDir = normalize(viewPos - FragPos);
    
    vec3 res = CalcDirLight(dirLight, norm, viewDir);

    for (int i = 0; i < nb_point_light; i++)
    {
        res += CalcPointLight(pointLights[i], norm, FragPos, viewDir);
    }
    res += CalcSpotLight(spotLight, norm, FragPos, viewDir);

    // Skybox reflection
    if (material.has_reflection) {
        float reflectFactor = 0.5;
        float refractFactor = 1.0 / 1.50;

        vec3 reflectDir = reflect(-viewDir, norm);
        vec3 refractDir = refract(reflectDir, norm, refractFactor);
        vec3 skyboxRefraction = texture(skybox, refractDir).rgb;
        
        res = mix(res, skyboxRefraction, reflectFactor);
    }

    if (material.has_emission) {
        res += vec3(texture(material.emission, TexCoords));
    }

    if (material.has_diffuse) {
        vec4 texture_color = texture(material.texture_diffuse, TexCoords);
        if (texture_color.a < 0.1) {
            discard;
        }
        float alpha = texture_color.a;
        colorOut.rgb = res * alpha;
        colorOut.a = alpha;
    } else {
        colorOut = texture(material.missing_texture, TexCoords);
    }
}

vec3 CalcDirLight(DirLight light, vec3 normal, vec3 viewDir)
{
    vec3 lightDir = normalize(-light.direction);

    // diffuse
    float diff = max(dot(normal, lightDir), 0.0);
    
    // specular
    vec3 reflectDir = reflect(-lightDir, normal);
    float spec = pow(max(dot(viewDir, reflectDir), 0.0), material.shininess);
    
    vec3 ambient  = light.ambient  * vec3(texture(material.texture_diffuse, TexCoords));
    vec3 diffuse  = light.diffuse  * diff * vec3(texture(material.texture_diffuse, TexCoords));

    vec3 specular = vec3(0.0);
    if (material.has_specular) {
        specular = light.specular * spec * vec3(texture(material.texture_specular, TexCoords));
    }
    
    return (ambient + diffuse + specular);
}

vec3 CalcPointLight(PointLight light, vec3 normal, vec3 fragPos, vec3 viewDir)
{
    vec3 lightDir = normalize(light.position - fragPos);
    
    // diffuse
    float diff = max(dot(normal, lightDir), 0.0);
    
    // specular
    vec3 reflectDir = reflect(-lightDir, normal);
    float spec = pow(max(dot(viewDir, reflectDir), 0.0), material.shininess);
    
    // attenuation
    float distance    = length(light.position - fragPos);
    float attenuation = 1.0 / (light.constant + light.linear * distance + 
  			     light.quadratic * (distance * distance));    
    
    vec3 ambient  = light.ambient  * vec3(texture(material.texture_diffuse, TexCoords));
    vec3 diffuse  = light.diffuse  * diff * vec3(texture(material.texture_diffuse, TexCoords));
    vec3 specular = light.specular * spec * vec3(texture(material.texture_specular, TexCoords));
    
    ambient  *= attenuation;
    diffuse  *= attenuation;
    specular *= attenuation;
    
    return (ambient + diffuse + specular);
}

vec3 CalcSpotLight(SpotLight light, vec3 normal, vec3 fragPos, vec3 viewDir)
{
    if (!light.isEnabled) {
        return vec3(0.0);
    }

    vec3 lightDir = normalize(light.position - fragPos);

    // diffuse
    float diff = max(dot(normal, lightDir), 0.0);

    // specular
    vec3 reflectDir = reflect(-lightDir, normal);
    float spec = pow(max(dot(viewDir, reflectDir), 0.0), material.shininess);

    // attenuation
    float distance = length(light.position - fragPos);
    float attenuation = 1.0 / (light.constant + light.linear * distance + light.quadratic * (distance * distance));

    // intensity
    float theta = dot(lightDir, normalize(-light.direction));
    float epsilon = light.cutOff - light.outerCutOff;
    float intensity = clamp((theta - light.outerCutOff) / epsilon, 0.0, 1.0);

    vec3 ambient  = light.ambient  * vec3(texture(material.texture_diffuse, TexCoords));
    vec3 diffuse  = light.diffuse  * diff * vec3(texture(material.texture_diffuse, TexCoords));

    vec3 specular = vec3(0.0);
    if (material.has_specular) {
        specular = light.specular * spec * vec3(texture(material.texture_specular, TexCoords));
    }
    
    ambient  *= attenuation * intensity;
    diffuse  *= attenuation * intensity;
    specular *= attenuation * intensity;

    return (ambient + diffuse + specular);
}
